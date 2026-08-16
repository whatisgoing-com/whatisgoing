#!/usr/bin/env bash
# Sets up (idempotent — safe to re-run) and starts the ner-sentiment
# service directly on the host instead of in Docker (issue #37). GLiNER +
# torch make the Docker image large and slow to iterate on locally; running
# natively means editing app/*.py takes effect on the next request with
# --reload, no image rebuild.
#
# First run downloads ~4-5GB of model weights (GLiNER, spaCy, the
# sentiment model, and the topic-generation model) — needs internet access
# once; cached under ~/.cache afterward, so later runs are fast and
# offline-capable.
set -euo pipefail
cd "$(dirname "$0")"

PYTHON_VERSION="3.11.11"

if [ -f .venv/bin/python ]; then
    PYTHON_BIN=.venv/bin/python
elif command -v pyenv >/dev/null 2>&1; then
    pyenv install --skip-existing "$PYTHON_VERSION"
    PYTHON_BIN="$(pyenv root)/versions/$PYTHON_VERSION/bin/python3"
elif command -v python3.11 >/dev/null 2>&1; then
    PYTHON_BIN="$(command -v python3.11)"
else
    echo "error: need Python 3.11 — install via 'pyenv install $PYTHON_VERSION' (pyenv not found on PATH), or install python3.11 directly. This service's torch/gliner pins need it; the system python3 here is too old." >&2
    exit 1
fi

if [ ! -d .venv ]; then
    echo "==> creating venv (.venv) with $PYTHON_BIN"
    "$PYTHON_BIN" -m venv .venv
fi

# shellcheck disable=SC1091
source .venv/bin/activate

echo "==> installing dependencies"
pip install --quiet --upgrade pip
# Plain PyPI torch (no --index-url) is correct here, unlike the Dockerfile:
# PyPI's default wheel is already CPU-only on macOS (no CUDA variant
# exists for it), and picks up Apple Silicon acceleration where available.
pip install --quiet torch
pip install --quiet -r requirements.txt

echo "==> downloading model weights (no-op if already cached)"
python -m spacy download en_core_web_sm
python -c "from transformers import pipeline; pipeline('sentiment-analysis', model='distilbert-base-uncased-finetuned-sst-2-english', device=-1)"
python -c "from gliner import GLiNER; GLiNER.from_pretrained('urchade/gliner_medium-v2.1')"
python -c "from huggingface_hub import hf_hub_download; hf_hub_download(repo_id='Qwen/Qwen2.5-1.5B-Instruct-GGUF', filename='qwen2.5-1.5b-instruct-q8_0.gguf')"

# transformers 5.x's tokenizer loader makes a live Hub call (listing repo
# files for a chat template) on every load regardless of what's already
# cached — silently defeating the point of predownloading above on any
# network hiccup. Force fully offline from here on; everything needed is
# already on disk.
export HF_HUB_OFFLINE=1

PORT="${PORT:-8000}"
echo "==> starting on :$PORT (--reload; edit app/*.py and it picks it up)"
exec uvicorn app.main:app --host 0.0.0.0 --port "$PORT" --reload
