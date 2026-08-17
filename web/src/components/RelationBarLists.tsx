import { Link } from 'react-router-dom'
import type { RelatedEntity } from '../api/types'

const SECTION_LABELS: Record<string, string> = {
  PERSON: 'Related people',
  ORG: 'Related organizations',
  TOPIC: 'Related topics',
}

const BAR_COLORS: Record<string, string> = {
  PERSON: 'bg-blue-500',
  ORG: 'bg-purple-500',
  TOPIC: 'bg-amber-500',
}

function groupByType(related: RelatedEntity[]): [string, RelatedEntity[]][] {
  const groups = new Map<string, RelatedEntity[]>()
  for (const entity of related) {
    const group = groups.get(entity.type)
    if (group) {
      group.push(entity)
    } else {
      groups.set(entity.type, [entity])
    }
  }
  return [...groups.entries()]
}

function RelationList({ type, entities }: { type: string; entities: RelatedEntity[] }) {
  const maxCount = Math.max(...entities.map((e) => e.cooccurrence_count))
  const barColor = BAR_COLORS[type] ?? 'bg-gray-400'

  return (
    <div>
      <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">{SECTION_LABELS[type] ?? `Related ${type.toLowerCase()}`}</h3>
      <ul className="space-y-2">
        {entities.map((entity) => (
          <li key={entity.id}>
            <Link to={`/entities/${entity.id}`} className="block">
              <div className="mb-1 flex items-baseline justify-between gap-2 text-sm">
                <span className="truncate text-gray-800 hover:underline">{entity.name}</span>
                <span className="shrink-0 tabular-nums text-gray-400">{entity.cooccurrence_count}</span>
              </div>
              <div className="h-1.5 w-full rounded-full bg-gray-100">
                <div className={`h-1.5 rounded-full ${barColor}`} style={{ width: `${(entity.cooccurrence_count / maxCount) * 100}%` }} />
              </div>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  )
}

// Horizontal weighted bar lists, split by entity type — replaces the
// radial relation graph (issue #62), which was hard to read once more
// than a handful of related entities showed up. Weight = shared-article
// count within the selected window (passed in already filtered by the
// caller).
export function RelationBarLists({ related }: { related: RelatedEntity[] }) {
  if (related.length === 0) {
    return <p className="py-4 text-center text-sm text-gray-500">No related entities in this window.</p>
  }

  const groups = groupByType(related)

  return (
    <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
      {groups.map(([type, entities]) => (
        <RelationList key={type} type={type} entities={entities} />
      ))}
    </div>
  )
}
