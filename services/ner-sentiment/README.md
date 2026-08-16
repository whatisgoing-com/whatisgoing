# ner-sentiment

Extracts entities (PERSON/TOPIC) and per-sentence sentiment from article text. A single `POST /extract` endpoint: `{"html": "...", "url": "..."}` → extracted plain text plus a flat list of entity mentions (character offsets into that text, sentiment score in `[-1, 1]`).

## Approach

- **Extraction**: `trafilatura` for full article pages; falls back to a plain tag-strip for HTML fragments too short for it to find boilerplate to remove from (common for RSS item content).
- **PERSON**: [GLiNER](https://huggingface.co/urchade/gliner_medium-v2.1) (`urchade/gliner_medium-v2.1`), a zero-shot NER model. ORG is deliberately not extracted right now (2026-08-16) — leaning on just PERSON and TOPIC for now, other types to come back later; not a quality problem with GLiNER's ORG output.
- **TOPIC**: generated, not extracted — see "Why generation, not extraction" below.
- **Sentiment**: `distilbert-base-uncased-finetuned-sst-2-english`, computed per sentence and attributed to every entity mentioned in that sentence (whole-article sentiment for the generated TOPIC, since it isn't scoped to one sentence).
- spaCy (`en_core_web_sm`) stays in the pipeline only for sentence segmentation, needed for the per-sentence sentiment above. It does no entity recognition or topic extraction itself.

### Why GLiNER for PERSON (issue #37)

The original implementation used spaCy's `en_core_web_sm` for NER. Real-data testing (25 real ingested articles, run through the actual extraction path) showed it mistagging obvious cases — `Kalshi`, `Alice Springs`, `Louis Vuitton` all came back as PERSON. GLiNER's zero-shot labeling got these right. Real per-article cost: roughly 0.6s on CPU once warmed up, vs. spaCy's near-instant — acceptable at this project's scale (~1,000 articles/day).

### Why generation, not extraction, for TOPIC (issue #37, round 2)

A topic is what an editor would call the article — not necessarily a phrase that appears verbatim in it. Two earlier approaches were both extractive (pick an existing span) and it showed: a noun-chunk-frequency heuristic let site boilerplate through ("breaking news email", image-caption fragments), and GLiNER's own "topic" label, while cleaner, was still fundamentally picking spans rather than characterizing the article.

Generating a short label instead needed a model that could run fast enough on CPU:

- **Qwen2.5-0.5B-Instruct** (plain `transformers`/PyTorch): fast (~1-2s/article) but unreliable — hallucinated "environment" as the topic for an unrelated article, and few-shot prompting didn't fix it.
- **Qwen2.5-1.5B-Instruct** (plain `transformers`/PyTorch): reliably good output, but 21-35s/article on unoptimized CPU fp32 inference — a non-starter at this project's volume.
- **The same Qwen2.5-1.5B-Instruct weights, quantized to GGUF and run via [llama.cpp](https://github.com/ggml-org/llama.cpp)** (`n_gpu_layers=0`, forced CPU-only so the numbers hold on the production box, which has no GPU): 0.4-1.2s/article standalone, with quality matching the slow fp32 run. The fix wasn't a smaller model — it was proper CPU inference. Plain PyTorch is known to be inefficient for CPU-only LLM inference; `llama.cpp`'s quantized kernels are built for exactly this.

The topic label is generated from the title + the article's lede (first ~1200 characters — news articles front-load the gist), asked for 2-3 words, no leading article. A 1.5B model doesn't hit exactly 2-3 words every time (occasionally one word), which is treated as acceptable variance rather than something to fight further.

### Concurrency: a single `Llama` instance isn't thread-safe

Found the hard way, not in the docs: a single `llama_cpp.Llama` instance's context (KV cache) isn't safe for concurrent `create_chat_completion` calls from multiple threads. Under real concurrent ingestion load (`cmd/core`'s bounded worker pool sends several `/extract` calls at once; FastAPI dispatches sync endpoints to a thread pool), this crashed the whole process outright — a `SIGSEGV` (confirmed via the macOS crash reporter, `EXC_BAD_ACCESS`/`KERN_INVALID_ADDRESS`), not a graceful error. Never showed up in sequential manual testing, only under real concurrency. Fixed with a `threading.Lock()` around the generation call (`_topic_generator_lock` in `nlp.py`) — GLiNER/spaCy/the sentiment pipeline don't need this, they have no shared mutable inference state.

Re-verified with 6 concurrent requests after the fix: no crash, 2.5-3.6s/article (vs. ~1.5-2.5s for an isolated single request) — the lock serializes topic generation specifically, so concurrent requests queue for that one step rather than running it in parallel; still an overall win since PERSON extraction/sentiment scoring for other requests can proceed while one holds the lock.

## Running it locally (not in Docker)

GLiNER + the topic model + torch make the Docker image large and slow to iterate on — editing `app/*.py` means a full image rebuild every time. Run it natively instead:

```sh
./run.sh
```

Idempotent — safe to re-run any time (e.g. after a `git pull` that touched `requirements.txt`). It:

1. Finds or provisions Python 3.11 (via `pyenv install`, falling back to a `python3.11` already on `PATH`) — the pinned `torch`/`gliner`/`llama-cpp-python` versions need it; the plain system `python3` on macOS is typically 3.9, which won't work.
2. Creates `.venv/` (skipped if it already exists) and installs `requirements.txt`.
3. Downloads model weights (GLiNER, spaCy's `en_core_web_sm`, the sentiment model, the quantized topic-generation model) — first run only, needs internet access; ~4-5GB, cached under `~/.cache` afterward.
4. Starts `uvicorn` on `:8000` with `--reload`, so editing `app/*.py` takes effect on the next request without restarting anything.

Override the port with `PORT=8001 ./run.sh`.

### How `core` reaches it

`cmd/core` still runs in docker-compose. It reaches this natively-running service via `http://host.docker.internal:8000` (`docker-compose.yml`'s `core.environment.NER_SENTIMENT_URL`) — that hostname is how a container reaches the host machine on Docker Desktop (macOS/Windows); `core`'s `extra_hosts: host.docker.internal:host-gateway` entry makes it resolve on Linux too. Start this service before `docker compose up core` (or restart `core` afterward) so its first request doesn't fail.

## Production (Docker)

`Dockerfile` still builds a real container image, used for CI/production (Harbor + k3s) — local dev is the only thing that moved off it. Model weights are baked in at build time the same way `run.sh` pre-downloads them, so the container needs no network access at runtime (verified by starting the built image with `--network none`).

`llama-cpp-python` isn't pure Python — it wraps the `llama.cpp` C++ project and needs to compile it if no prebuilt wheel matches the build platform, which can take several minutes (`cmake` is in the image for exactly this, alongside `build-essential` for spaCy's own occasional from-source builds).

**Open question, not yet resolved**: local testing of the *built Docker image* (not `run.sh`) showed real additional slowdown beyond the native numbers above — a fresh process inside the container took ~6-8s/article (vs. ~1.5-2.5s natively), and the long-running server process was slower still (~12-20s first request, ~3-8s once warm). This was measured on macOS Docker Desktop, whose virtualized VM/file-sharing layer is a plausible cause (loading a 1.6GB memory-mapped GGUF file through it) — but that's unverified, since this project hasn't been deployed to real server hardware yet. Worth a real measurement once there's an actual k3s box to test on, before assuming production performance matches local dev.

## Testing

```sh
source .venv/bin/activate   # after running ./run.sh at least once, so model weights are cached
pip install -r requirements-dev.txt
pytest tests/ -v
```

Tests exercise the real models in-process (`TestClient`, no mocking) — no separately running server needed, just the venv with weights already cached from a prior `./run.sh` run.
