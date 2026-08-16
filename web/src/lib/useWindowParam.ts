import { useSearchParams } from 'react-router-dom'
import type { Window } from '../api/types'

const VALID_WINDOWS = new Set<string>(['day', 'week', 'month', 'year'])

export function useWindowParam(): Window {
  const [params] = useSearchParams()
  const raw = params.get('window')
  return raw && VALID_WINDOWS.has(raw) ? (raw as Window) : 'day'
}
