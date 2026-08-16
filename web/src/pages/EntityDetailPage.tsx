import { Link, useParams } from 'react-router-dom'
import { useEntityDetail, useRecentArticles, useRelatedEntities, useSourceBreakdown } from '../api/hooks'
import { ApiRequestError } from '../api/client'
import { RecentArticles } from '../components/RecentArticles'
import { RelationGraph } from '../components/RelationGraph'
import { SentimentGauge } from '../components/SentimentGauge'
import { SentimentPieChart } from '../components/SentimentPieChart'
import { SourceBreakdown } from '../components/SourceBreakdown'
import { TrendChart } from '../components/TrendChart'
import { TypeBadge } from '../components/TypeBadge'
import { WindowTabs } from '../components/WindowTabs'
import { useWindowParam } from '../lib/useWindowParam'

export function EntityDetailPage() {
  const { id: idParam } = useParams<{ id: string }>()
  const id = Number(idParam)
  const window = useWindowParam()

  const detail = useEntityDetail(id, window)
  const sources = useSourceBreakdown(id)
  const recentArticles = useRecentArticles(id)
  const related = useRelatedEntities(id)

  if (detail.isError) {
    const notFound = detail.error instanceof ApiRequestError && detail.error.status === 404
    return (
      <>
        <p>
          <Link to="/" className="text-sm text-blue-600 hover:underline">
            &larr; Trending
          </Link>
        </p>
        <h1 className="text-2xl font-bold tracking-tight">{notFound ? 'Not found' : 'Something went wrong'}</h1>
        <p className="text-gray-500">{notFound ? "No such entity, or it hasn't been through a rollup yet." : detail.error.message}</p>
      </>
    )
  }

  if (!detail.data) return null

  const trend = detail.data.trend
  const chartData = trend.map((p) => ({ label: p.window_start, mentions: p.mention_count, sentiment: p.sentiment_score }))
  const latest = trend[trend.length - 1]

  return (
    <>
      <p>
        <Link to="/" className="text-sm text-blue-600 hover:underline">
          &larr; Trending
        </Link>
      </p>
      <div className="flex items-center gap-2">
        <h1 className="text-2xl font-bold tracking-tight">{detail.data.name}</h1>
        <TypeBadge type={detail.data.type} />
      </div>
      {detail.data.description && (
        <p className="max-w-2xl text-sm text-gray-600">
          {detail.data.description} <span className="text-gray-400">(source: Wikipedia)</span>
        </p>
      )}

      <WindowTabs active={window} basePath={`/entities/${id}`} />

      <section className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="rounded-xl border border-gray-200 bg-white p-4">
          <h2 className="mb-2 text-sm font-semibold text-gray-700">Mentions &amp; sentiment over time</h2>
          <div className="h-64">
            <TrendChart data={chartData} />
          </div>
        </div>
        <div className="rounded-xl border border-gray-200 bg-white p-4">
          <h2 className="mb-2 text-sm font-semibold text-gray-700">Sentiment breakdown (latest window)</h2>
          <div className="h-64">
            <SentimentPieChart
              slices={[
                { name: 'Positive', value: latest?.positive_count ?? 0, color: '#16a34a' },
                { name: 'Neutral', value: latest?.neutral_count ?? 0, color: '#9ca3af' },
                { name: 'Negative', value: latest?.negative_count ?? 0, color: '#dc2626' },
              ]}
            />
          </div>
        </div>
      </section>

      <section className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="rounded-xl border border-gray-200 bg-white p-4">
          <h2 className="mb-3 text-sm font-semibold text-gray-700">By source</h2>
          <SourceBreakdown sources={sources.data ?? []} />
        </div>
        <div className="rounded-xl border border-gray-200 bg-white p-4">
          <h2 className="mb-1 text-sm font-semibold text-gray-700">Recent articles</h2>
          <RecentArticles articles={recentArticles.data ?? []} />
        </div>
      </section>

      <section className="rounded-xl border border-gray-200 bg-white p-4">
        <div className="mb-1 flex items-baseline justify-between">
          <h2 className="text-sm font-semibold text-gray-700">Related entities</h2>
          <span className="text-xs text-gray-400">Edge thickness = shared articles</span>
        </div>
        <RelationGraph centerName={detail.data.name} related={related.data ?? []} />
      </section>

      <div className="overflow-x-auto rounded-xl border border-gray-200 bg-white p-4">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-gray-200 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
              <th className="py-2 pr-4">Date</th>
              <th className="py-2 pr-4 text-right">Mentions</th>
              <th className="py-2 text-right">Sentiment</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {trend.map((point) => (
              <tr key={point.window_start}>
                <td className="py-2 pr-4">{point.window_start}</td>
                <td className="py-2 pr-4 text-right tabular-nums">{point.mention_count}</td>
                <td className="py-2 text-right">
                  <SentimentGauge score={point.sentiment_score} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}
