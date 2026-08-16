import { Link } from 'react-router-dom'
import type { EntityRollup } from '../api/types'
import { SentimentGauge } from './SentimentGauge'

export function EntityRankList({ entities }: { entities: EntityRollup[] }) {
  if (entities.length === 0) {
    return <p className="py-4 text-center text-sm text-gray-500">No trending entities yet.</p>
  }

  const max = entities.reduce((m, e) => Math.max(m, e.mention_count), 0)

  return (
    <ol className="divide-y divide-gray-100">
      {entities.map((entity, i) => (
        <li key={entity.id} className="flex items-start gap-3 py-2.5">
          <span className="w-4 shrink-0 pt-0.5 text-right text-xs font-semibold tabular-nums text-gray-300">{i + 1}</span>
          <div className="min-w-0 flex-1">
            <Link to={`/entities/${entity.id}`} className="block text-sm font-medium text-gray-900 hover:text-blue-600">
              {entity.name}
            </Link>
            <div className="mt-1 h-1 w-full max-w-xs rounded-full bg-gray-100">
              <div className="h-1 rounded-full bg-blue-500" style={{ width: `${max > 0 ? (entity.mention_count * 100) / max : 0}%` }} />
            </div>
          </div>
          <span className="shrink-0 text-xs font-medium tabular-nums text-gray-500">{entity.mention_count}</span>
          <SentimentGauge score={entity.sentiment_score} />
        </li>
      ))}
    </ol>
  )
}
