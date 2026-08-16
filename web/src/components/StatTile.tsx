import type { ReactNode } from 'react'

export function StatTile({ value, label, valueClassName = 'text-gray-900' }: { value: ReactNode; label: string; valueClassName?: string }) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 text-center">
      <div className={`text-2xl font-bold ${valueClassName}`}>{value}</div>
      <div className="text-xs font-medium uppercase tracking-wide text-gray-500">{label}</div>
    </div>
  )
}
