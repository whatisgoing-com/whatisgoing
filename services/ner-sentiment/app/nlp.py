from typing import Dict, List, Optional, Tuple

import spacy
from gliner import GLiNER
from spacy.tokens import Doc, Span
from transformers import pipeline

from .schemas import EntityMention

# GLiNER extracts PERSON/ORG/TOPIC directly via zero-shot labels (issue
# #37) — a real side-by-side against 25 real ingested articles showed it
# clearly more accurate on PERSON/ORG than spaCy's en_core_web_sm NER
# (which mistagged e.g. "Kalshi"/"Alice Springs"/"Louis Vuitton" as
# PERSON), and its topic candidates need none of the noun-chunk heuristic's
# boilerplate leakage ("breaking news email", "Image credit" bled through
# from RSS captions in the old approach).
_GLINER_MODEL = "urchade/gliner_medium-v2.1"
_GLINER_LABELS = ["person", "organization", "topic"]
_GLINER_LABEL_TO_TYPE = {"person": "PERSON", "organization": "ORG", "topic": "TOPIC"}
# The eval ran at 0.4 and still let noise through ("tensions" 0.45,
# "variables" 0.46) — 0.5 (GLiNER's own library default) cuts most of that
# while leaving real topics ("bike lanes" 0.74, "cartel violence" 0.77)
# comfortably clear of the bar.
_GLINER_THRESHOLD = 0.5
_TOPIC_LIMIT = 3

_gliner = GLiNER.from_pretrained(_GLINER_MODEL)

# spaCy no longer does entity recognition — GLiNER replaced it — but stays
# loaded for two things GLiNER doesn't do: sentence segmentation (sentiment
# is scored per sentence, see below) and a GPE/LOC safety net excluding
# bare countries/places from topic candidates, since GLiNER's "topic" label
# alone isn't a hard guarantee against ever surfacing one.
_nlp = spacy.load("en_core_web_sm")
_TOPIC_EXCLUDED_LABELS = {"GPE", "LOC"}

_sentiment = pipeline(
    "sentiment-analysis",
    model="distilbert-base-uncased-finetuned-sst-2-english",
    device=-1,
)


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


def _overlaps_any(start: int, end: int, spans: List[Tuple[int, int]]) -> bool:
    return any(start < seen_end and seen_start < end for seen_start, seen_end in spans)


def _longest_noun_chunk(doc: Doc) -> Optional[Span]:
    return max(doc.noun_chunks, key=lambda chunk: len(chunk.text), default=None)


def _trim_leading_article(doc: Doc, start: int, end: int) -> Tuple[int, int]:
    """GLiNER's topic spans sometimes keep a leading article ("a second
    mass crossing", "The 55.6-metre concrete figure") that the old
    noun-chunk heuristic used to strip. Only applied to topics, not
    PERSON/ORG — a leading "The" is sometimes part of a real name ("The
    Guardian", "The Beatles"), never part of a topic phrase. Reuses
    spaCy's tokenization (already loaded for sentence segmentation)
    rather than a separate regex/stopword list."""
    span = doc.char_span(start, end, alignment_mode="expand")
    if span is None:
        return start, end
    tokens = list(span)
    i = 0
    while i < len(tokens) - 1 and (tokens[i].pos_ in {"DET", "PRON"} or tokens[i].is_punct):
        i += 1
    if i == 0:
        return start, end
    return tokens[i].idx, end


def analyze_entities(text: str) -> List[EntityMention]:
    doc = _nlp(text)

    sentence_scores: Dict[Tuple[int, int], float] = {}
    mentions: List[EntityMention] = []
    entity_spans: List[Tuple[int, int]] = []
    topic_excluded_spans: List[Tuple[int, int]] = [
        (ent.start_char, ent.end_char) for ent in doc.ents if ent.label_ in _TOPIC_EXCLUDED_LABELS
    ]

    topic_candidates: List[Tuple[int, int]] = []
    for gent in _gliner.predict_entities(text, _GLINER_LABELS, threshold=_GLINER_THRESHOLD):
        entity_type = _GLINER_LABEL_TO_TYPE[gent["label"]]
        start, end = gent["start"], gent["end"]

        if entity_type == "TOPIC":
            topic_candidates.append(_trim_leading_article(doc, start, end))
            continue

        entity_spans.append((start, end))
        mentions.append(_make_mention(doc, text, start, end, entity_type, sentence_scores))

    topic_count = 0
    excluded = entity_spans + topic_excluded_spans
    for start, end in topic_candidates:
        if topic_count >= _TOPIC_LIMIT:
            break
        if _overlaps_any(start, end, excluded):
            continue
        mentions.append(_make_mention(doc, text, start, end, "TOPIC", sentence_scores))
        topic_count += 1

    # Every article should get at least one topic (issue #39). GLiNER
    # doesn't guarantee that — thin, single-sentence RSS content (common in
    # this dataset) often reads as one unified idea rather than a separable
    # named topic, so this fallback triggers more often than the fuller
    # articles in the original eval suggested — so fall back to the longest
    # noun chunk, same last-resort this service used before GLiNER.
    if topic_count == 0:
        chunk = _longest_noun_chunk(doc)
        if chunk is not None:
            start, end = _trim_leading_article(doc, chunk.start_char, chunk.end_char)
            mentions.append(_make_mention(doc, text, start, end, "TOPIC", sentence_scores))

    return mentions
