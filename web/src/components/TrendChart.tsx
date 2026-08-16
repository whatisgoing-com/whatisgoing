import { CartesianGrid, ComposedChart, Legend, Line, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts'

export interface TrendChartPoint {
  label: string
  mentions: number
  sentiment: number
}

export function TrendChart({ data }: { data: TrendChartPoint[] }) {
  return (
    <ResponsiveContainer width="100%" height="100%">
      <ComposedChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#f3f4f6" />
        <XAxis dataKey="label" tick={{ fontSize: 11 }} stroke="#9ca3af" />
        <YAxis yAxisId="mentions" tick={{ fontSize: 11 }} stroke="#9ca3af" allowDecimals={false} />
        <YAxis yAxisId="sentiment" orientation="right" domain={[-1, 1]} tick={{ fontSize: 11 }} stroke="#9ca3af" />
        <Tooltip contentStyle={{ fontSize: 12 }} />
        <Legend wrapperStyle={{ fontSize: 12 }} />
        <Line yAxisId="mentions" type="monotone" dataKey="mentions" name="Mentions" stroke="#3b82f6" dot={false} strokeWidth={2} />
        <Line yAxisId="sentiment" type="monotone" dataKey="sentiment" name="Avg sentiment" stroke="#16a34a" dot={false} strokeWidth={2} />
      </ComposedChart>
    </ResponsiveContainer>
  )
}
