# ner-sentiment

Extracts entities (PERSON/ORG/TOPIC) and per-sentence sentiment from article text. A single `POST /extract` endpoint: `{"html": "...", "url": "..."}` → extracted plain text plus a flat list of entity mentions (character offsets into that text, sentiment score in `[-1, 1]`).

## Approach

- **Extraction**: `trafilatura` for full article pages; falls back to a plain tag-strip for HTML fragments too short for it to find boilerplate to remove from (common for RSS item content).
- **Entities**: [GLiNER](https://huggingface.co/urchade/gliner_medium-v2.1) (`urchade/gliner_medium-v2.1`), a zero-shot NER model — extracts PERSON, ORG, and TOPIC directly via labeled prompts, rather than a fixed-label model plus a hand-rolled topic heuristic (see "Why GLiNER" below).
- **Sentiment**: `distilbert-base-uncased-finetuned-sst-2-english`, computed per sentence and attributed to every entity mentioned in that sentence — not whole-article sentiment.
- spaCy (`en_core_web_sm`) stays in the pipeline for two things GLiNER doesn't do: sentence segmentation (needed for the per-sentence sentiment above) and a GPE/LOC safety net excluding bare countries/places from topic candidates. It no longer does entity recognition itself.

### Why GLiNER (issue #37)

The original implementation used spaCy's `en_core_web_sm` for PERSON/ORG, plus a hand-rolled noun-chunk-frequency heuristic for TOPIC. Real-data testing (25 real ingested articles, run through the actual extraction path) showed the small spaCy model mistagging obvious cases — `Kalshi`, `Alice Springs`, `Louis Vuitton` all came back as PERSON — and the topic heuristic let site boilerplate through ("breaking news email", image-caption fragments). GLiNER's zero-shot labeling fixed both: it got the mistagged cases right, and its topic candidates need no length heuristic to avoid junk, since the label itself is semantic — a confident single word like "sports" is a real topic; a low-confidence guess isn't, regardless of length.

Trade-off: GLiNER is a real transformer (a few hundred MB of weights) vs. spaCy's small model, and costs real per-article latency — roughly 0.6s on CPU once warmed up, vs. spaCy's near-instant. Acceptable at this project's scale (~1,000 articles/day).

GLiNER doesn't guarantee at least one topic per article the way the old heuristic's "longest noun chunk" fallback did — a real eval found ~2/25 articles (short, thin content) got none. That fallback is still here, just demoted to a true last resort.

## Running it locally (not in Docker)

GLiNER + torch make the Docker image large and slow to iterate on — editing `app/*.py` means a full image rebuild every time. Run it natively instead:

```sh
./run.sh
```

Idempotent — safe to re-run any time (e.g. after a `git pull` that touched `requirements.txt`). It:

1. Finds or provisions Python 3.11 (via `pyenv install`, falling back to a `python3.11` already on `PATH`) — the pinned `torch`/`gliner` versions need it; the plain system `python3` on macOS is typically 3.9, which won't work.
2. Creates `.venv/` (skipped if it already exists) and installs `requirements.txt`.
3. Downloads model weights (GLiNER, spaCy's `en_core_web_sm`, the sentiment model) — first run only, needs internet access; a few GB, cached under `~/.cache` afterward.
4. Starts `uvicorn` on `:8000` with `--reload`, so editing `app/*.py` takes effect on the next request without restarting anything.

Override the port with `PORT=8001 ./run.sh`.

### How `core` reaches it

`cmd/core` still runs in docker-compose. It reaches this natively-running service via `http://host.docker.internal:8000` (`docker-compose.yml`'s `core.environment.NER_SENTIMENT_URL`) — that hostname is how a container reaches the host machine on Docker Desktop (macOS/Windows); `core`'s `extra_hosts: host.docker.internal:host-gateway` entry makes it resolve on Linux too. Start this service before `docker compose up core` (or restart `core` afterward) so its first request doesn't fail.

## Production (Docker)

`Dockerfile` still builds a real container image, used for CI/production (Harbor + k3s) — local dev is the only thing that moved off it. Model weights are baked in at build time the same way `run.sh` pre-downloads them, so the container needs no network access at runtime.

## Testing

```sh
source .venv/bin/activate   # after running ./run.sh at least once, so model weights are cached
pip install -r requirements-dev.txt
pytest tests/ -v
```

Tests exercise the real models in-process (`TestClient`, no mocking) — no separately running server needed, just the venv with weights already cached from a prior `./run.sh` run.
