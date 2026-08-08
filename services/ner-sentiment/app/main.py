import logging
import time

from fastapi import FastAPI, HTTPException

from .extraction import extract_article
from .nlp import analyze_entities
from .schemas import ExtractRequest, ExtractResponse

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("ner-sentiment")

app = FastAPI(title="whatisgoing ner-sentiment service")


@app.get("/healthz")
def healthz():
    return {"status": "ok"}


@app.post("/extract", response_model=ExtractResponse)
def extract(req: ExtractRequest) -> ExtractResponse:
    started = time.perf_counter()

    title, text = extract_article(req.html, req.url)
    if not text:
        raise HTTPException(status_code=400, detail="no extractable text content")

    entities = analyze_entities(text)

    processing_ms = (time.perf_counter() - started) * 1000
    logger.info(
        "processed article url=%s chars=%d entities=%d in %.1fms",
        req.url,
        len(text),
        len(entities),
        processing_ms,
    )

    return ExtractResponse(title=title, text=text, entities=entities, processing_ms=processing_ms)
