import { Link } from 'react-router-dom'
import type { Window } from '../api/types'

const TABS: { value: Window; label: string }[] = [
  { value: 'day', label: 'Today' },
  { value: 'week', label: 'This week' },
  { value: 'month', label: 'This month' },
  { value: 'year', label: 'This year' },
]

export function WindowTabs({ active, basePath, rangeLabel }: { active: Window; basePath: string; rangeLabel?: string }) {
  return (
    <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-b border-gray-200 pb-0">
      <div className="flex gap-1">
        {TABS.map((tab) => (
          <Link
            key={tab.value}
            to={`${basePath}?window=${tab.value}`}
            className={`-mb-px border-b-2 px-3 py-2 text-sm font-medium ${
              tab.value === active ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700'
            }`}
          >
            {tab.label}
          </Link>
        ))}
      </div>
      {rangeLabel && <span className="pb-2 text-xs text-gray-500">{rangeLabel}</span>}
    </div>
  )
}
