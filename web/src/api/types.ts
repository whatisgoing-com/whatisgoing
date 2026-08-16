// Mirrors internal/core/api's JSON DTOs field-for-field (see dto.go and
// the individual handlers) — kept snake_case to match the wire format
// exactly rather than adding a mapping layer for a handful of fields.

export type EntityType = 'PERSON' | 'ORG' | 'TOPIC'
export type Window = 'day' | 'week' | 'month' | 'year'

export interface EntityRollup {
  id: number
  name: string
  type: EntityType
  mention_count: number
  sentiment_score: number
  window_start: string
  positive_count: number
  neutral_count: number
  negative_count: number
}

export interface OverallTrendPoint {
  window_start: string
  total_mentions: number
  avg_sentiment: number
}

export interface SentimentBreakdown {
  positive: number
  neutral: number
  negative: number
}

export interface WindowStats {
  article_count: number
  entity_count: number
  window_start: string
  window_end: string
}

export interface EntityDetail {
  id: number
  name: string
  type: EntityType
  description?: string
  trend: EntityRollup[]
}

export interface SourceBreakdown {
  source_id: string
  source_name: string
  mention_count: number
  avg_sentiment: number
}

export interface RelatedEntity {
  id: number
  name: string
  type: EntityType
  description?: string
  cooccurrence_count: number
}

export interface RecentArticle {
  id: number
  title: string
  url: string
  source_name: string
  published_at: string
}

export interface EntitySummary {
  id: number
  name: string
  type: EntityType
}

export interface SearchResult {
  id: string
  url: string
  title: string
  source_id: string
  published_at: string
}

export interface ApiError {
  error: string
}
