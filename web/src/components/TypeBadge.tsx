const STYLES: Record<string, string> = {
  PERSON: 'bg-blue-50 text-blue-700',
  ORG: 'bg-purple-50 text-purple-700',
  TOPIC: 'bg-amber-50 text-amber-700',
}

export function TypeBadge({ type }: { type: string }) {
  return (
    <span className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${STYLES[type] ?? STYLES.TOPIC}`}>
      {type}
    </span>
  )
}
