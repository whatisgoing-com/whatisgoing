import { useQuery } from '@tanstack/react-query'
import { api } from './client'
import type { EntityType, Window } from './types'

export function useTrending(window: Window, type: EntityType) {
  return useQuery({ queryKey: ['trending', window, type], queryFn: () => api.trending(window, type) })
}

export function useOverallTrend(window: Window, limit: number) {
  return useQuery({ queryKey: ['overallTrend', window, limit], queryFn: () => api.overallTrend(window, limit) })
}

export function useSentimentBreakdown(window: Window) {
  return useQuery({ queryKey: ['sentimentBreakdown', window], queryFn: () => api.sentimentBreakdown(window) })
}

export function useWindowStats(window: Window) {
  return useQuery({ queryKey: ['windowStats', window], queryFn: () => api.windowStats(window) })
}

export function useRecentArticles(entityId?: number) {
  return useQuery({ queryKey: ['recentArticles', entityId ?? null], queryFn: () => api.recentArticles(entityId) })
}

export function useEntityDetail(id: number, window: Window) {
  return useQuery({ queryKey: ['entityDetail', id, window], queryFn: () => api.entityDetail(id, window), retry: false })
}

export function useSourceBreakdown(id: number) {
  return useQuery({ queryKey: ['sourceBreakdown', id], queryFn: () => api.sourceBreakdown(id) })
}

export function useRelatedEntities(id: number) {
  return useQuery({ queryKey: ['relatedEntities', id], queryFn: () => api.relatedEntities(id) })
}

export function useEntitySearch(query: string) {
  return useQuery({
    queryKey: ['entitySearch', query],
    queryFn: () => api.entitySearch(query),
    enabled: query.trim().length > 0,
  })
}

export function useArticleSearch(query: string) {
  return useQuery({
    queryKey: ['search', query],
    queryFn: () => api.search(query),
    enabled: query.trim().length > 0,
  })
}
