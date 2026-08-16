import threading
from typing import Dict, List, Optional, Tuple

import spacy
from gliner import GLiNER
from huggingface_hub import hf_hub_download
from llama_cpp import Llama
from spacy.tokens import Doc, Span
from transformers import pipeline

from .schemas import EntityMention

# GLiNER extracts PERSON via zero-shot labels (issue #37) — a real
# side-by-side against 25 real ingested articles showed it clearly more
# accurate than spaCy's en_core_web_sm NER, which mistagged e.g.
# "Kalshi"/"Alice Springs"/"Louis Vuitton" as PERSON. ORG was dropped
# deliberately (2026-08-16): the plan is to lean on just PERSON and TOPIC
# for now and reintroduce ORG (or other types) later once those two are
# solid — not a quality problem with GLiNER's ORG output specifically.
# The Postgres entity_type enum still has ORG (harmless to leave unused,
# an enum value can't be cheaply dropped anyway) — this is a pure
# application-level change, no migration.
_GLINER_MODEL = "urchade/gliner_medium-v2.1"
_GLINER_ENTITY_LABELS = ["person"]
_GLINER_LABEL_TO_TYPE = {"person": "PERSON"}
_GLINER_THRESHOLD = 0.5

_gliner = GLiNER.from_pretrained(_GLINER_MODEL)

# spaCy no longer does entity recognition or topic extraction — it's
# loaded only for sentence segmentation, so PERSON sentiment can be
# attributed to the sentence a mention actually appeared in.
_nlp = spacy.load("en_core_web_sm")

_sentiment = pipeline(
    "sentiment-analysis",
    model="distilbert-base-uncased-finetuned-sst-2-english",
    device=-1,
)

# Topic generation (issue #37, round 2): a topic is what an editor would
# call the article, not necessarily a phrase that appears verbatim in it
# — the previous approaches (noun-chunk frequency, then GLiNER's own
# "topic" label) were both fundamentally extractive, which is the wrong
# shape for this.
#
# A plain transformers/PyTorch pipeline was tried first and rejected:
# Qwen2.5-0.5B-Instruct was fast (~1-2s/article) but unreliable (hallucinated
# "environment" for an unrelated article); Qwen2.5-1.5B-Instruct was
# reliably good but 21-35s/article on unoptimized CPU fp32 inference —
# a non-starter at this project's ~1,000 articles/day volume. The fix
# wasn't a smaller model, it was proper CPU inference: the same
# Qwen2.5-1.5B-Instruct weights, quantized to GGUF and run via llama.cpp
# (n_gpu_layers=0, forced CPU-only so these numbers hold on the
# production box, which has no GPU), came back at 0.4-1.2s/article with
# quality matching the slow fp32 run.
_TOPIC_MODEL_REPO = "Qwen/Qwen2.5-1.5B-Instruct-GGUF"
_TOPIC_MODEL_FILE = "qwen2.5-1.5b-instruct-q8_0.gguf"
_TOPIC_ARTICLE_CHARS = 1200
_TOPIC_MAX_TOKENS = 16
_TOPIC_SYSTEM_PROMPT = (
    "You are a news editor. Given an article's title and text, reply with "
    "ONLY a concise topic label. Always use exactly 2 or 3 words — never a "
    "single word. Like a section tag, not a sentence. No punctuation, no "
    "quotation marks, no leading article (\"a\"/\"the\")."
)

# Llama.from_pretrained(repo_id=..., filename=...) always calls the Hub API
# to resolve the filename glob, even when the file is already cached —
# unlike transformers' from_pretrained, it has no offline-if-cached path,
# so it hard-fails under HF_HUB_OFFLINE. hf_hub_download does respect the
# local cache correctly, so resolve the path that way and load from it.
_topic_generator = Llama(
    model_path=hf_hub_download(repo_id=_TOPIC_MODEL_REPO, filename=_TOPIC_MODEL_FILE),
    n_gpu_layers=0,
    n_ctx=1024,
    verbose=False,
)
# A single Llama instance's context (KV cache) isn't safe for concurrent
# generation calls from multiple threads — confirmed the hard way: this
# crashed the whole process with a SIGSEGV under real concurrent ingestion
# load (FastAPI's threadpool dispatches concurrent requests to sync
# endpoints), never during sequential manual testing. GLiNER/spaCy/the
# sentiment pipeline don't need this — plain inference calls with no
# shared mutable context.
_topic_generator_lock = threading.Lock()


def _sentence_sentiment_score(text: str) -> float:
    result = _sentiment(text, truncation=True, max_length=512)[0]
    score = result["score"]
    return score if result["label"] == "POSITIVE" else -score


def _sentence_for_offset(doc: Doc, start: int, end: int) -> Span:
    span = doc.char_span(start, end, alignment_mode="expand")
    if span is not None:
        return span.sent
    for sent in doc.sents:
        if sent.start_char <= start < sent.end_char:
            return sent
    return list(doc.sents)[-1]


def _make_mention(
    doc: Doc, text: str, start: int, end: int, entity_type: str, sentence_scores: Dict[Tuple[int, int], float]
) -> EntityMention:
    sent = _sentence_for_offset(doc, start, end)
    key = (sent.start_char, sent.end_char)
    if key not in sentence_scores:
        sentence_scores[key] = _sentence_sentiment_score(sent.text)

    return EntityMention(
        text=text[start:end],
        type=entity_type,
        start=start,
        end=end,
        sentiment_score=round(sentence_scores[key], 4),
    )


def _generate_topic(title: Optional[str], text: str) -> Optional[str]:
    """Generates a short (2-3 word) topic label from the title + the
    article's lede — news articles front-load the gist, so the opening
    chars are representative without needing the full body, which also
    bounds worst-case prompt length/latency for long articles."""
    body = text[:_TOPIC_ARTICLE_CHARS]
    messages = [
        {"role": "system", "content": _TOPIC_SYSTEM_PROMPT},
        {"role": "user", "content": f"Title: {title or '(none)'}\nArticle: {body}\nTopic:"},
    ]
    with _topic_generator_lock:
        result = _topic_generator.create_chat_completion(messages=messages, max_tokens=_TOPIC_MAX_TOKENS, temperature=0.0)
    generated = result["choices"][0]["message"]["content"] or ""
    topic = generated.strip().strip("\"'").rstrip(".").strip()
    return topic or None


def analyze_entities(text: str, title: Optional[str] = None) -> List[EntityMention]:
    doc = _nlp(text)

    sentence_scores: Dict[Tuple[int, int], float] = {}
    mentions: List[EntityMention] = []

    for gent in _gliner.predict_entities(text, _GLINER_ENTITY_LABELS, threshold=_GLINER_THRESHOLD):
        entity_type = _GLINER_LABEL_TO_TYPE[gent["label"]]
        mentions.append(_make_mention(doc, text, gent["start"], gent["end"], entity_type, sentence_scores))

    topic = _generate_topic(title, text)
    if topic:
        # The topic is generated, not extracted, so it has no natural span
        # in the article — start/end are 0 rather than a fabricated
        # position; nothing downstream reads them for TOPIC mentions (the
        # Go store layer never reads a mention's offsets at all, see
        # internal/core/store/postgres/store.go). Sentiment is the whole
        # article's, not one sentence's, since the topic isn't scoped to
        # a single sentence either.
        mentions.append(
            EntityMention(
                text=topic,
                type="TOPIC",
                start=0,
                end=0,
                sentiment_score=round(_sentence_sentiment_score(text), 4),
            )
        )

    return mentions
