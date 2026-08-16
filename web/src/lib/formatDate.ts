// Articles are recent enough by construction that the year is rarely
// useful — ported from cmd/ui/templates.go's formatPublishedAt.
export function formatPublishedAt(raw: string): string {
  const d = new Date(raw)
  if (Number.isNaN(d.getTime())) return raw
  const month = d.toLocaleString('en-US', { month: 'short' })
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${month} ${d.getDate()}, ${hh}:${mm}`
}
