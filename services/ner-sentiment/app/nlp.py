from typing import Dict, List, Tuple

import spacy
from transformers import pipeline

from .schemas import EntityMention

_ENTITY_WHITELIST = {"PERSON", "ORG", "EVENT"}

_nlp = spacy.load("en_core_web_sm")
_sentiment = pipeline(
    "sentiment-analysis",
    model="distilbert-base-uncased-finetuned-sst-2-english",
    device=-1,
)


def _sentence_sentiment_score(text: str) -> float:
    result = _sentiment(text, truncation=True, max_length=512)[0]
    score = result["score"]
    return score if result["label"] == "POSITIVE" else -score


def analyze_entities(text: str) -> List[EntityMention]:
    doc = _nlp(text)

    sentence_scores: Dict[Tuple[int, int], float] = {}
    mentions: List[EntityMention] = []

    for ent in doc.ents:
        if ent.label_ not in _ENTITY_WHITELIST:
            continue

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

    return mentions
