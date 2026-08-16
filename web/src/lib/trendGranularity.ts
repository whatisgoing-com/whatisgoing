import type { Window } from '../api/types'

// Ported from cmd/ui/main.go's trendGranularity (issue #44): each window
// tab shows one grain finer than itself, over a lookback long enough to
// read as a trend but short enough to stay legible — entity_rollups
// already stores all four grains every rollup run, so this is a pure
// display-layer choice, not a query concern.
export function trendGranularity(window: Window): { grain: Window; limit: number } {
  switch (window) {
    case 'year':
      return { grain: 'month', limit: 12 }
    case 'month':
      return { grain: 'week', limit: 5 }
    case 'week':
    case 'day':
    default:
      return { grain: 'day', limit: 7 }
  }
}
