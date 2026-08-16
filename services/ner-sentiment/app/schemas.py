from typing import List, Literal, Optional

from pydantic import BaseModel, Field

EntityType = Literal["PERSON", "ORG", "TOPIC"]


class ExtractRequest(BaseModel):
    html: str = Field(..., min_length=1, description="Raw HTML or RSS item content to extract from")
    url: Optional[str] = Field(None, description="Original article URL; used only as an extraction hint, never fetched")


class EntityMention(BaseModel):
    text: str
    type: EntityType
    start: int
    end: int
    sentiment_score: float = Field(
        ...,
        ge=-1.0,
        le=1.0,
        description="Sentence-level sentiment attributed to this mention; positive is favorable, negative is unfavorable",
    )


class ExtractResponse(BaseModel):
    title: Optional[str] = None
    text: str
    entities: List[EntityMention]
    processing_ms: float
