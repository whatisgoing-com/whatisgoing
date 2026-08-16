import type { Window } from '../api/types'

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const MONTHS_LONG = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
]
const WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

// Parses a "2006-01-02"-style date-only string as a UTC calendar date, not
// a local-timezone instant — these represent window boundaries from core's
// API, not moments in time, so shifting them by the viewer's timezone would
// silently move the displayed range by a day near midnight.
function parseDateOnly(raw: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(raw)
  if (!match) return null
  const [, y, m, d] = match
  return new Date(Date.UTC(Number(y), Number(m) - 1, Number(d)))
}

// Ported from cmd/ui/templates.go's formatWindowRangeLabel: turns a
// window's real start/end (windowEnd is exclusive) into a short human
// label, e.g. "Aug 15, 2026" for a day, "Aug 10 – 16, 2026" for a week.
export function formatWindowRangeLabel(window: Window, windowStart: string, windowEnd: string): string {
  const start = parseDateOnly(windowStart)
  const end = parseDateOnly(windowEnd)
  if (!start || !end) return ''

  const lastDay = new Date(end)
  lastDay.setUTCDate(lastDay.getUTCDate() - 1)

  switch (window) {
    case 'day':
      return `${WEEKDAYS[start.getUTCDay()]}, ${MONTHS[start.getUTCMonth()]} ${start.getUTCDate()}, ${start.getUTCFullYear()}`
    case 'week':
      if (start.getUTCMonth() === lastDay.getUTCMonth()) {
        return `${MONTHS[start.getUTCMonth()]} ${start.getUTCDate()}–${lastDay.getUTCDate()}, ${start.getUTCFullYear()}`
      }
      return `${MONTHS[start.getUTCMonth()]} ${start.getUTCDate()} – ${MONTHS[lastDay.getUTCMonth()]} ${lastDay.getUTCDate()}, ${lastDay.getUTCFullYear()}`
    case 'month':
      return `${MONTHS_LONG[start.getUTCMonth()]} ${start.getUTCFullYear()}`
    case 'year':
      return `${start.getUTCFullYear()}`
    default:
      return ''
  }
}
