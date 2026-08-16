import { Cell, Legend, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts'

export interface PieSlice {
  name: string
  value: number
  color: string
}

export function SentimentPieChart({ slices }: { slices: PieSlice[] }) {
  const hasData = slices.some((s) => s.value > 0)
  if (!hasData) {
    return <div className="flex h-full items-center justify-center text-sm text-gray-500">No sentiment data yet.</div>
  }

  return (
    <ResponsiveContainer width="100%" height="100%">
      <PieChart>
        <Pie data={slices} dataKey="value" nameKey="name" innerRadius={50} outerRadius={80} paddingAngle={2}>
          {slices.map((s) => (
            <Cell key={s.name} fill={s.color} />
          ))}
        </Pie>
        <Tooltip contentStyle={{ fontSize: 12 }} />
        <Legend wrapperStyle={{ fontSize: 12 }} />
      </PieChart>
    </ResponsiveContainer>
  )
}
