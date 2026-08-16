// Diverging bar gauge (issue #43): a visual read on sentiment reads far
// faster than a raw -1..1 number. Fill grows from the center in either
// direction, capped at half the bar's width each way.
function fillPct(score: number): number {
  let pct = Math.round(score * 50)
  if (pct < 0) pct = -pct
  if (pct > 50) pct = 50
  return pct
}

export function SentimentGauge({ score }: { score: number }) {
  const pct = fillPct(score)
  return (
    <span className="relative inline-block h-1.5 w-14 shrink-0 rounded-full bg-gray-100 align-middle" title={score.toFixed(2)}>
      <span className="absolute inset-y-0 left-1/2 w-px bg-gray-300" />
      {score > 0 && <span className="absolute inset-y-0 left-1/2 rounded-r-full bg-green-500" style={{ width: `${pct}%` }} />}
      {score < 0 && <span className="absolute inset-y-0 rounded-l-full bg-red-500" style={{ right: '50%', width: `${pct}%` }} />}
    </span>
  )
}
