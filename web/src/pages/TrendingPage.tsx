import { EntitySearch } from '../components/EntitySearch'
import { EntityRankList } from '../components/EntityRankList'
import { RecentArticles } from '../components/RecentArticles'
import { SentimentPieChart } from '../components/SentimentPieChart'
import { StatTile } from '../components/StatTile'
import { SentimentGauge } from '../components/SentimentGauge'
import { TrendChart } from '../components/TrendChart'
import { WindowTabs } from '../components/WindowTabs'
import { useOverallTrend, useRecentArticles, useSentimentBreakdown, useTrending, useWindowStats } from '../api/hooks'
import { trendGranularity } from '../lib/trendGranularity'
import { formatWindowRangeLabel } from '../lib/windowRangeLabel'
import { useWindowParam } from '../lib/useWindowParam'

export function TrendingPage() {
  const window = useWindowParam()
  const { grain, limit } = trendGranularity(window)

  // ORG isn't extracted right now (2026-08-16) — leaning on PERSON and
  // TOPIC for now, ORG (and other types) to come back later.
  const persons = useTrending(window, 'PERSON')
  const topics = useTrending(window, 'TOPIC')
  const overall = useOverallTrend(grain, limit)
  const sentiment = useSentimentBreakdown(window)
  const stats = useWindowStats(window)
  const recentArticles = useRecentArticles()

  const rangeLabel = stats.data ? formatWindowRangeLabel(window, stats.data.window_start, stats.data.window_end) : ''

  // The current window's real aggregate sentiment — the same number the
  // time-series chart's most recent point shows — not derived from only
  // the top-N trending entities (ported from cmd/ui/templates.go).
  const avgSentiment = overall.data && overall.data.length > 0 ? overall.data[overall.data.length - 1].avg_sentiment : 0

  const chartData = (overall.data ?? []).map((p) => ({
    label: p.window_start,
    mentions: p.total_mentions,
    sentiment: p.avg_sentiment,
  }))

  return (
    <>
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-2xl font-bold tracking-tight">Trending</h1>
        <EntitySearch />
      </div>

      <div className="space-y-6">
        <WindowTabs active={window} basePath="/" rangeLabel={rangeLabel} />

        <section className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          <StatTile value={stats.data?.article_count ?? '–'} label="Articles" />
          <StatTile value={stats.data?.entity_count ?? '–'} label="Entities mentioned" />
          <StatTile value={sentiment.data?.positive ?? '–'} label="Positive" valueClassName="text-green-600" />
          <StatTile value={sentiment.data?.negative ?? '–'} label="Negative" valueClassName="text-red-600" />
          <div className="rounded-xl border border-gray-200 bg-white p-4 text-center">
            <div className="flex justify-center">
              <SentimentGauge score={avgSentiment} />
            </div>
            <div className="mt-2 text-xs font-medium uppercase tracking-wide text-gray-500">Avg sentiment</div>
          </div>
        </section>

        <section className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          <div className="rounded-xl border border-gray-200 bg-white p-4">
            <h2 className="mb-2 text-sm font-semibold text-gray-700">Mentions &amp; sentiment over time</h2>
            <div className="h-64">
              <TrendChart data={chartData} />
            </div>
          </div>
          <div className="rounded-xl border border-gray-200 bg-white p-4">
            <h2 className="mb-2 text-sm font-semibold text-gray-700">Overall sentiment breakdown</h2>
            <div className="h-64">
              <SentimentPieChart
                slices={[
                  { name: 'Positive', value: sentiment.data?.positive ?? 0, color: '#16a34a' },
                  { name: 'Negative', value: sentiment.data?.negative ?? 0, color: '#dc2626' },
                ]}
              />
            </div>
          </div>
        </section>

        <section className="space-y-4">
          <div className="rounded-xl border border-gray-200 bg-white p-4">
            <h2 className="mb-1 text-sm font-semibold text-gray-700">Top persons</h2>
            <EntityRankList entities={persons.data ?? []} />
          </div>
          <div className="rounded-xl border border-gray-200 bg-white p-4">
            <h2 className="mb-1 text-sm font-semibold text-gray-700">Top topics</h2>
            <EntityRankList entities={topics.data ?? []} />
          </div>
        </section>

        <section className="rounded-xl border border-gray-200 bg-white p-4">
          <h2 className="mb-1 text-sm font-semibold text-gray-700">Recent articles</h2>
          <RecentArticles articles={recentArticles.data ?? []} />
        </section>
      </div>
    </>
  )
}
