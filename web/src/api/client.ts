import type {
  ApiError,
  EntityDetail,
  EntityRollup,
  EntitySummary,
  OverallTrendPoint,
  RecentArticle,
  RelatedEntity,
  SearchResult,
  SentimentBreakdown,
  SourceBreakdown,
  Window,
  WindowStats,
} from './types'

// The browser calls core's API directly (no BFF in between anymore), so
// this needs a URL it can actually reach — not the docker-network-internal
// "core" hostname cmd/ui used server-side. Configurable at build time via
// VITE_CORE_API_URL for non-local deployments.
const BASE_URL = import.meta.env.VITE_CORE_API_URL ?? 'http://localhost:8080'

export class ApiRequestError extends Error {
  status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiRequestError'
    this.status = status
  }
}

async function getJSON<T>(path: string, params?: Record<string, string | number | undefined>): Promise<T> {
  const url = new URL(path, BASE_URL)
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined) url.searchParams.set(key, String(value))
    }
  }

  const res = await fetch(url)
  if (!res.ok) {
    const body: Partial<ApiError> = await res.json().catch(() => ({}))
    throw new ApiRequestError(body.error ?? `request to ${path} failed: ${res.status}`, res.status)
  }
  return res.json()
}

export const api = {
  trending: (window: Window, type?: EntitySummary['type'], limit = 10) =>
    getJSON<EntityRollup[]>('/api/trending', { window, type, limit }),

  overallTrend: (window: Window, limit: number) => getJSON<OverallTrendPoint[]>('/api/trend/overall', { window, limit }),

  entitySearch: (q: string, limit = 10) => getJSON<EntitySummary[]>('/api/entities', { q, limit }),

  entityDetail: (id: number, window: Window) => getJSON<EntityDetail>(`/api/entities/${id}`, { window }),

  search: (q: string, limit = 20) => getJSON<SearchResult[]>('/api/search', { q, limit }),

  sentimentBreakdown: (window: Window) => getJSON<SentimentBreakdown>('/api/sentiment', { window }),

  windowStats: (window: Window) => getJSON<WindowStats>('/api/stats', { window }),

  sourceBreakdown: (id: number) => getJSON<SourceBreakdown[]>(`/api/entities/${id}/sources`),

  relatedEntities: (id: number, limit = 10) => getJSON<RelatedEntity[]>(`/api/entities/${id}/related`, { limit }),

  recentArticles: (entityId?: number, limit = 10) => getJSON<RecentArticle[]>('/api/articles/recent', { entity_id: entityId, limit }),
}
