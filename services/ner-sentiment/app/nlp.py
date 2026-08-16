from typing import Dict, List, Optional, Tuple

import spacy
from spacy.tokens import Doc, Span
from transformers import pipeline

from .schemas import EntityMention

_ENTITY_WHITELIST = {"PERSON", "ORG"}
# Not emitted as mentions, but excluded from topic candidates — a bare
# country/city/region name is a place, not a topic (issue #41).
_TOPIC_EXCLUDED_LABELS = {"GPE", "LOC"}

_nlp = spacy.load("en_core_web_sm")
_sentiment = pipeline(
    "sentiment-analysis",
    model="distilbert-base-uncased-finetuned-sst-2-english",
    device=-1,
)

# Topic extraction for TOPIC entities (issue #39). spaCy's own EVENT NER
# label was extraction noise — it rarely found a real event and polluted
# the entity list. Ranking spaCy's noun chunks by in-article frequency is
# CPU-only, needs no model weights or training data (fits this cluster's
# no-GPU constraint — see the sentiment pipeline above), and — unlike a
# raw n-gram keyword extractor — is grammatically guaranteed to produce
# real noun phrases rather than word salad crossing clause boundaries.
_TOPIC_LIMIT = 3
_TOPIC_MIN_CHARS = 3


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


def _topic_mention(doc: Doc, text: str, start: int, end: int, sentence_scores: Dict[Tuple[int, int], float]) -> EntityMention:
    sent = _sentence_for_offset(doc, start, end)
    key = (sent.start_char, sent.end_char)
    if key not in sentence_scores:
        sentence_scores[key] = _sentence_sentiment_score(sent.text)

    return EntityMention(
        text=text[start:end],
        type="TOPIC",
        start=start,
        end=end,
        sentiment_score=round(sentence_scores[key], 4),
    )


def _longest_noun_chunk(doc: Doc) -> Optional[Span]:
    return max(doc.noun_chunks, key=lambda chunk: len(chunk.text), default=None)


def _trim_chunk(chunk: Span) -> Optional[Span]:
    """Strips leading determiners/pronouns and trailing stopwords/punct off
    a noun chunk, so "the risks" and "its innovation" read as bare topics
    ("risks", "innovation") rather than including an article."""
    tokens = list(chunk)
    start_i = 0
    while start_i < len(tokens) and (tokens[start_i].pos_ in {"DET", "PRON"} or tokens[start_i].is_stop or tokens[start_i].is_punct):
        start_i += 1
    end_i = len(tokens)
    while end_i > start_i and (tokens[end_i - 1].is_stop or tokens[end_i - 1].is_punct):
        end_i -= 1
    if start_i >= end_i:
        return None
    trimmed_tokens = tokens[start_i:end_i]
    # A single word — common noun or proper noun alike ("court", "company",
    # "Bangladesh", "Russia") — is too generic, or is just a bare name/place,
    # to read as a real topic. A topic is a theme/subject, which needs at
    # least a modifier + head noun ("large language models", "ongoing
    # conflict") to say anything.
    if len(trimmed_tokens) < 2:
        return None
    return chunk.doc[trimmed_tokens[0].i : trimmed_tokens[-1].i + 1]


def _overlaps_any(start: int, end: int, spans: List[Tuple[int, int]]) -> bool:
    return any(start < seen_end and seen_start < end for seen_start, seen_end in spans)


def _extract_topics(
    doc: Doc,
    sentence_scores: Dict[Tuple[int, int], float],
    entity_spans: List[Tuple[int, int]],
    excluded_spans: List[Tuple[int, int]],
) -> List[EntityMention]:
    """Ranks the article's noun chunks by how often they recur (a phrase
    mentioned repeatedly is what the article is actually about), skipping
    anything that overlaps a PERSON/ORG mention already found, or a
    GPE/LOC (country/city/region) span — a topic restating a named entity
    or a bare place name isn't a topic. Every article gets at least one
    TOPIC mention: if nothing clears the bar (e.g. unusually short text),
    the longest noun chunk is used as a fallback."""
    counts: Dict[str, int] = {}
    first_span: Dict[str, Span] = {}
    for chunk in doc.noun_chunks:
        trimmed = _trim_chunk(chunk)
        if trimmed is None or len(trimmed.text) < _TOPIC_MIN_CHARS:
            continue
        if _overlaps_any(trimmed.start_char, trimmed.end_char, excluded_spans):
            continue

        key = trimmed.text.lower()
        counts[key] = counts.get(key, 0) + 1
        first_span.setdefault(key, trimmed)

    ranked = sorted(first_span, key=lambda k: (-counts[k], -len(k)))

    mentions: List[EntityMention] = []
    seen_spans: List[Tuple[int, int]] = list(entity_spans)
    for key in ranked:
        if len(mentions) >= _TOPIC_LIMIT:
            break

        span = first_span[key]
        start, end = span.start_char, span.end_char
        if _overlaps_any(start, end, seen_spans):
            continue

        seen_spans.append((start, end))
        mentions.append(_topic_mention(doc, doc.text, start, end, sentence_scores))

    if not mentions:
        chunk = _longest_noun_chunk(doc)
        if chunk is not None:
            mentions.append(_topic_mention(doc, doc.text, chunk.start_char, chunk.end_char, sentence_scores))

    return mentions


def analyze_entities(text: str) -> List[EntityMention]:
    doc = _nlp(text)

    sentence_scores: Dict[Tuple[int, int], float] = {}
    mentions: List[EntityMention] = []
    entity_spans: List[Tuple[int, int]] = []
    topic_excluded_spans: List[Tuple[int, int]] = []

    for ent in doc.ents:
        if ent.label_ in _TOPIC_EXCLUDED_LABELS:
            topic_excluded_spans.append((ent.start_char, ent.end_char))

        if ent.label_ not in _ENTITY_WHITELIST:
            continue

        entity_spans.append((ent.start_char, ent.end_char))

        sent = ent.sent
        key = (sent.start_char, sent.end_char)
        if key not in sentence_scores:
            sentence_scores[key] = _sentence_sentiment_score(sent.text)

        mentions.append(
            EntityMention(
                text=ent.text,
                type=ent.label_,
                start=ent.start_char,
                end=ent.end_char,
                sentiment_score=round(sentence_scores[key], 4),
            )
        )

    mentions.extend(_extract_topics(doc, sentence_scores, entity_spans, entity_spans + topic_excluded_spans))

    return mentions
