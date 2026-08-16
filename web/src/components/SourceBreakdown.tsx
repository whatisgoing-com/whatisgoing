import type { SourceBreakdown as SourceBreakdownRow } from '../api/types'
import { SentimentGauge } from './SentimentGauge'

export function SourceBreakdown({ sources }: { sources: SourceBreakdownRow[] }) {
  if (sources.length === 0) {
    return <p className="text-sm text-gray-500">No source data yet.</p>
  }

  const max = sources.reduce((m, s) => Math.max(m, s.mention_count), 0)

  return (
    <ul className="space-y-3">
      {sources.map((s) => (
        <li key={s.source_id}>
          <div className="flex items-center justify-between text-sm">
            <span className="font-medium text-gray-900">{s.source_name}</span>
            <span className="flex items-center gap-2">
              <span className="tabular-nums text-gray-500">{s.mention_count}</span>
              <SentimentGauge score={s.avg_sentiment} />
            </span>
          </div>
          <div className="mt-1 h-1.5 w-full rounded-full bg-gray-100">
            <div className="h-1.5 rounded-full bg-blue-500" style={{ width: `${max > 0 ? (s.mention_count * 100) / max : 0}%` }} />
          </div>
        </li>
      ))}
    </ul>
  )
}
