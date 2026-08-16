FROM python:3.11-slim
WORKDIR /app

# build-essential is needed because spaCy has no prebuilt wheel for every
# platform/Python combo and falls back to compiling from source; cmake is
# for the same reason with llama-cpp-python if no matching prebuilt wheel
# exists for this platform.
RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential cmake \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir torch --index-url https://download.pytorch.org/whl/cpu \
    && pip install --no-cache-dir -r requirements.txt \
    && python -m spacy download en_core_web_sm

# Pre-download sentiment, GLiNER, and topic-generation model weights at
# build time so the container never needs network access at runtime.
RUN python -c "from transformers import pipeline; pipeline('sentiment-analysis', model='distilbert-base-uncased-finetuned-sst-2-english', device=-1)" \
    && python -c "from gliner import GLiNER; GLiNER.from_pretrained('urchade/gliner_medium-v2.1')" \
    && python -c "from huggingface_hub import hf_hub_download; hf_hub_download(repo_id='Qwen/Qwen2.5-1.5B-Instruct-GGUF', filename='qwen2.5-1.5b-instruct-q8_0.gguf')"

# transformers 5.x's tokenizer loader makes a live Hub call (listing repo
# files for a chat template) on every load regardless of what's already
# cached, which would otherwise reintroduce a runtime network dependency
# despite the predownload above.
ENV HF_HUB_OFFLINE=1

COPY app ./app
EXPOSE 8000
CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
