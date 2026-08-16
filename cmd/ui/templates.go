package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/ui/coreclient"
)

const layoutHeader = `<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>whatisgoing.com</title>
	<link rel="icon" type="image/svg+xml" href="/static/favicon.svg">
	<link rel="stylesheet" href="/static/style.css">
	<script src="/static/htmx.min.js"></script>
	<script src="/static/chart.min.js"></script>
</head>
<body class="bg-gray-50 text-gray-900 antialiased">
	<header class="border-b border-gray-200 bg-white">
		<div class="mx-auto flex max-w-5xl items-center justify-between px-4 py-4">
			<a href="/" class="font-brand text-xl font-extrabold tracking-tight text-[#1C2430]">what is<span class="rounded-[3px] bg-[#1C2430] px-1 text-[#F6F3EC]">going</span><span class="font-normal text-gray-400">.com</span></a>
			<nav class="flex gap-1">
				<a href="/" class="rounded-md px-3 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900">Trending</a>
				<a href="/search" class="rounded-md px-3 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900">Search</a>
			</nav>
		</div>
	</header>
	<main class="mx-auto max-w-5xl space-y-6 px-4 py-8">
`

const layoutFooter = `
	</main>
</body>
</html>
`

var tmpl = template.Must(template.New("ui").Funcs(template.FuncMap{
	"sentimentFillPct": sentimentFillPct,
}).Parse(`
{{define "sentimentGauge"}}
<span class="relative inline-block h-1.5 w-14 shrink-0 rounded-full bg-gray-100 align-middle" title="{{printf "%.2f" .}}">
	<span class="absolute inset-y-0 left-1/2 w-px bg-gray-300"></span>
	{{if gt . 0.0}}<span class="absolute inset-y-0 left-1/2 rounded-r-full bg-green-500" style="width: {{sentimentFillPct .}}%"></span>
	{{else if lt . 0.0}}<span class="absolute inset-y-0 rounded-l-full bg-red-500" style="right: 50%; width: {{sentimentFillPct .}}%"></span>
	{{end}}
</span>
{{end}}

{{define "typeBadge"}}
{{if eq . "PERSON"}}<span class="inline-block rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">PERSON</span>
{{else if eq . "ORG"}}<span class="inline-block rounded-full bg-purple-50 px-2 py-0.5 text-xs font-medium text-purple-700">ORG</span>
{{else if eq . "TOPIC"}}<span class="inline-block rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700">TOPIC</span>
{{else}}<span class="inline-block rounded-full bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700">{{.}}</span>
{{end}}
{{end}}

{{define "recentArticles"}}
{{if .}}
<ul class="divide-y divide-gray-100">
	{{range .}}
	<li class="py-2.5">
		<a href="{{.URL}}" target="_blank" rel="noopener" class="font-medium text-gray-900 hover:text-blue-600">{{.Title}}</a>
		<div class="text-xs text-gray-500">{{.SourceName}} &middot; {{.PublishedAt}}</div>
	</li>
	{{end}}
</ul>
{{else}}
<p class="text-sm text-gray-500">No articles yet.</p>
{{end}}
{{end}}

{{define "trendingContent"}}
<div class="flex flex-wrap items-center justify-between gap-4">
	<h1 class="text-2xl font-bold tracking-tight">Trending</h1>
	<div class="entity-search relative w-full sm:w-72">
		<input type="search" placeholder="Find an entity&hellip;" autocomplete="off"
			class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
			hx-get="/entities/search" hx-trigger="keyup changed delay:300ms, search" hx-target="#entity-search-results" name="q">
		<div id="entity-search-results" class="absolute z-10 mt-1 w-full"></div>
	</div>
</div>

<div id="trending-panel" class="space-y-6">
{{template "trendingPanel" .}}
</div>
{{end}}

{{define "trendingPanel"}}
<div class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-b border-gray-200 pb-0">
	<div class="flex gap-1">
		{{range .Windows}}<a href="/?window={{.Value}}" hx-get="/?window={{.Value}}" hx-target="#trending-panel" hx-push-url="true"
			class="-mb-px border-b-2 px-3 py-2 text-sm font-medium {{if .Active}}border-blue-600 text-blue-600{{else}}border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700{{end}}">{{.Label}}</a>{{end}}
	</div>
	<span class="pb-2 text-xs text-gray-500">{{.WindowRangeLabel}}</span>
</div>

<section class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
	<div class="rounded-xl border border-gray-200 bg-white p-4 text-center">
		<div class="text-2xl font-bold text-gray-900">{{.ArticleCount}}</div>
		<div class="text-xs font-medium uppercase tracking-wide text-gray-500">Articles</div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4 text-center">
		<div class="text-2xl font-bold text-gray-900">{{.EntityCount}}</div>
		<div class="text-xs font-medium uppercase tracking-wide text-gray-500">Entities mentioned</div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4 text-center">
		<div class="text-2xl font-bold text-green-600">{{.Sentiment.Positive}}</div>
		<div class="text-xs font-medium uppercase tracking-wide text-gray-500">Positive</div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4 text-center">
		<div class="text-2xl font-bold text-red-600">{{.Sentiment.Negative}}</div>
		<div class="text-xs font-medium uppercase tracking-wide text-gray-500">Negative</div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4 text-center">
		<div class="flex justify-center">{{template "sentimentGauge" .Sentiment.Average}}</div>
		<div class="mt-2 text-xs font-medium uppercase tracking-wide text-gray-500">Avg sentiment</div>
	</div>
</section>

<section class="grid grid-cols-1 gap-4 lg:grid-cols-2">
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-2 text-sm font-semibold text-gray-700">Mentions &amp; sentiment over time</h2>
		<div class="h-64"><canvas id="trend-chart"></canvas></div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-2 text-sm font-semibold text-gray-700">Overall sentiment breakdown</h2>
		<div class="h-64"><canvas id="sentiment-pie-chart"></canvas></div>
	</div>
</section>

<section class="space-y-4">
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-1 text-sm font-semibold text-gray-700">Top persons</h2>
		{{template "entityRankList" .TopPersons}}
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-1 text-sm font-semibold text-gray-700">Top orgs</h2>
		{{template "entityRankList" .TopOrgs}}
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-1 text-sm font-semibold text-gray-700">Top topics</h2>
		{{template "entityRankList" .TopTopics}}
	</div>
</section>

<section class="rounded-xl border border-gray-200 bg-white p-4">
	<h2 class="mb-1 text-sm font-semibold text-gray-700">Recent articles</h2>
	{{template "recentArticles" .RecentArticles}}
</section>

<script>
(function() {
	const chartData = {{.ChartDataJSON}};
	new Chart(document.getElementById('trend-chart'), {
		type: 'line',
		data: {
			labels: chartData.trendLabels,
			datasets: [
				{ label: 'Mentions', data: chartData.trendMentions, borderColor: '#3b82f6', yAxisID: 'y' },
				{ label: 'Avg sentiment', data: chartData.trendSentiment, borderColor: '#16a34a', yAxisID: 'y1' }
			]
		},
		options: {
			maintainAspectRatio: false,
			scales: {
				y: { type: 'linear', position: 'left', beginAtZero: true },
				y1: { type: 'linear', position: 'right', min: -1, max: 1, grid: { drawOnChartArea: false } }
			}
		}
	});
	new Chart(document.getElementById('sentiment-pie-chart'), {
		type: 'pie',
		data: {
			labels: ['Positive', 'Negative'],
			datasets: [{ data: [chartData.sentimentPositive, chartData.sentimentNegative], backgroundColor: ['#16a34a', '#dc2626'] }]
		},
		options: { maintainAspectRatio: false }
	});
})();
</script>
{{end}}

{{define "entityRankList"}}
{{if .}}
<ol class="divide-y divide-gray-100">
	{{range .}}
	<li class="flex items-start gap-3 py-2.5">
		<span class="w-4 shrink-0 pt-0.5 text-right text-xs font-semibold tabular-nums text-gray-300">{{.Rank}}</span>
		<div class="min-w-0 flex-1">
			<a href="/entities/{{.ID}}" class="block text-sm font-medium text-gray-900 hover:text-blue-600">{{.Name}}</a>
			<div class="mt-1 h-1 w-full max-w-xs rounded-full bg-gray-100">
				<div class="h-1 rounded-full bg-blue-500" style="width: {{.BarPercent}}%"></div>
			</div>
		</div>
		<span class="shrink-0 text-xs font-medium tabular-nums text-gray-500">{{.MentionCount}}</span>
		{{template "sentimentGauge" .SentimentScore}}
	</li>
	{{end}}
</ol>
{{else}}
<p class="py-4 text-center text-sm text-gray-500">No trending entities yet.</p>
{{end}}
{{end}}

{{define "entitySearchResults"}}
{{if .}}
<ul class="divide-y divide-gray-100 rounded-lg border border-gray-200 bg-white shadow-lg">
	{{range .}}
		<li><a href="/entities/{{.ID}}" class="flex items-center justify-between px-3 py-2 text-sm hover:bg-gray-50"><span class="font-medium text-gray-900">{{.Name}}</span>{{template "typeBadge" .Type}}</a></li>
	{{end}}
</ul>
{{end}}
{{end}}

{{define "sourceBreakdown"}}
{{if .}}
<ul class="space-y-3">
	{{range .}}
	<li>
		<div class="flex items-center justify-between text-sm">
			<span class="font-medium text-gray-900">{{.SourceName}}</span>
			<span class="flex items-center gap-2">
				<span class="tabular-nums text-gray-500">{{.MentionCount}}</span>
				{{template "sentimentGauge" .AvgSentiment}}
			</span>
		</div>
		<div class="mt-1 h-1.5 w-full rounded-full bg-gray-100">
			<div class="h-1.5 rounded-full bg-blue-500" style="width: {{.BarPercent}}%"></div>
		</div>
	</li>
	{{end}}
</ul>
{{else}}
<p class="text-sm text-gray-500">No source data yet.</p>
{{end}}
{{end}}

{{define "relatedEntities"}}
{{if .}}
<ul class="divide-y divide-gray-100">
	{{range .}}
	<li class="flex items-center justify-between py-2 text-sm">
		<a href="/entities/{{.ID}}" class="font-medium text-gray-900 hover:text-blue-600">{{.Name}}</a>
		<span class="flex items-center gap-2">{{template "typeBadge" .Type}}<span class="tabular-nums text-gray-500">{{.CooccurrenceCount}}</span></span>
	</li>
	{{end}}
</ul>
{{else}}
<p class="text-sm text-gray-500">No related entities yet.</p>
{{end}}
{{end}}

{{define "entityContent"}}
<p><a href="/" class="text-sm text-blue-600 hover:underline">&larr; Trending</a></p>
<div class="flex items-center gap-2">
	<h1 class="text-2xl font-bold tracking-tight">{{.Detail.Name}}</h1>
	{{template "typeBadge" .Detail.Type}}
</div>
{{if .Detail.Description}}
<p class="max-w-2xl text-sm text-gray-600">{{.Detail.Description}} <span class="text-gray-400">(source: Wikipedia)</span></p>
{{end}}

<section class="grid grid-cols-1 gap-4 lg:grid-cols-2">
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-2 text-sm font-semibold text-gray-700">Mentions &amp; sentiment over time</h2>
		<div class="h-64"><canvas id="entity-trend-chart"></canvas></div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-2 text-sm font-semibold text-gray-700">Sentiment breakdown (latest window)</h2>
		<div class="h-64"><canvas id="entity-sentiment-pie-chart"></canvas></div>
	</div>
</section>

<section class="grid grid-cols-1 gap-4 lg:grid-cols-3">
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-3 text-sm font-semibold text-gray-700">By source</h2>
		{{template "sourceBreakdown" .SourceBreakdown}}
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-1 text-sm font-semibold text-gray-700">Recent articles</h2>
		{{template "recentArticles" .RecentArticles}}
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-3 text-sm font-semibold text-gray-700">Related entities</h2>
		{{template "relatedEntities" .RelatedEntities}}
	</div>
</section>

<div class="overflow-x-auto rounded-xl border border-gray-200 bg-white p-4">
<table class="w-full text-sm">
	<thead>
		<tr class="border-b border-gray-200 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
			<th class="py-2 pr-4">Date</th><th class="py-2 pr-4 text-right">Mentions</th><th class="py-2 text-right">Sentiment</th>
		</tr>
	</thead>
	<tbody class="divide-y divide-gray-100">
	{{range .Detail.Trend}}
		<tr>
			<td class="py-2 pr-4">{{.WindowStart}}</td>
			<td class="py-2 pr-4 text-right tabular-nums">{{.MentionCount}}</td>
			<td class="py-2 text-right">{{template "sentimentGauge" .SentimentScore}}</td>
		</tr>
	{{end}}
	</tbody>
</table>
</div>
<script>
(function() {
	const chartData = {{.ChartDataJSON}};
	new Chart(document.getElementById('entity-trend-chart'), {
		type: 'line',
		data: {
			labels: chartData.labels,
			datasets: [
				{ label: 'Mentions', data: chartData.mentions, borderColor: '#3b82f6', yAxisID: 'y' },
				{ label: 'Sentiment', data: chartData.sentiment, borderColor: '#16a34a', yAxisID: 'y1' }
			]
		},
		options: {
			maintainAspectRatio: false,
			scales: {
				y: { type: 'linear', position: 'left', beginAtZero: true },
				y1: { type: 'linear', position: 'right', min: -1, max: 1, grid: { drawOnChartArea: false } }
			}
		}
	});
	new Chart(document.getElementById('entity-sentiment-pie-chart'), {
		type: 'pie',
		data: {
			labels: ['Positive', 'Neutral', 'Negative'],
			datasets: [{ data: [chartData.sentimentPositive, chartData.sentimentNeutral, chartData.sentimentNegative], backgroundColor: ['#16a34a', '#9ca3af', '#dc2626'] }]
		},
		options: { maintainAspectRatio: false }
	});
})();
</script>
{{end}}

{{define "entityNotFound"}}
<p><a href="/" class="text-sm text-blue-600 hover:underline">&larr; Trending</a></p>
<h1 class="text-2xl font-bold tracking-tight">Not found</h1>
<p class="text-gray-500">No such entity, or it hasn't been through a rollup yet.</p>
{{end}}

{{define "searchContent"}}
<h1 class="text-2xl font-bold tracking-tight">Search</h1>
<form hx-get="/search" hx-target="#search-results" hx-push-url="true" class="flex gap-2">
	<input type="search" name="q" value="{{.Query}}" placeholder="Search articles&hellip;" autofocus
		class="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500">
	<button type="submit" class="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">Search</button>
</form>
<div id="search-results" class="rounded-xl border border-gray-200 bg-white p-4">
{{template "searchResults" .}}
</div>
{{end}}

{{define "searchResults"}}
{{if .Searched}}
<ul class="divide-y divide-gray-100">
	{{range .Results}}
		<li class="py-2.5"><a href="{{.URL}}" target="_blank" rel="noopener" class="font-medium text-gray-900 hover:text-blue-600">{{.Title}}</a><div class="text-xs text-gray-500">{{.PublishedAt}}</div></li>
	{{else}}
		<li class="py-4 text-center text-gray-500">No results.</li>
	{{end}}
</ul>
{{end}}
{{end}}
`))

type windowTab struct {
	Value  string
	Label  string
	Active bool
}

var windowTabs = []struct {
	Value string
	Label string
}{
	{"day", "Today"},
	{"week", "This week"},
	{"month", "This month"},
	{"year", "This year"},
}

// sentimentFillPct converts a -1..1 sentiment score into a 0..50 fill
// percentage for one half of the sentimentGauge template's diverging bar
// (the other half stays empty, since the bar's center is 0, not the
// score's own magnitude scaled across the full width).
func sentimentFillPct(score float64) int {
	pct := int(score * 50)
	if pct < 0 {
		pct = -pct
	}
	if pct > 50 {
		pct = 50
	}
	return pct
}

func tabsFor(active string) []windowTab {
	tabs := make([]windowTab, 0, len(windowTabs))
	for _, w := range windowTabs {
		tabs = append(tabs, windowTab{Value: w.Value, Label: w.Label, Active: w.Value == active})
	}
	return tabs
}

// Neutral is deliberately not tracked here: the sentiment model
// (distilbert-base-uncased-finetuned-sst-2-english) is a binary
// positive/negative classifier, so a mention's sentiment_score is never
// meaningfully zero — a "Neutral" tile would show 0 essentially always.
type sentimentSummary struct {
	Positive int
	Negative int
	Average  float64
}

// recentArticle is the UI-side view of coreclient.RecentArticle, with
// PublishedAt pre-formatted for display rather than left as raw RFC3339.
type recentArticle struct {
	Title       string
	URL         string
	SourceName  string
	PublishedAt string
}

func toRecentArticles(articles []coreclient.RecentArticle) []recentArticle {
	out := make([]recentArticle, len(articles))
	for i, a := range articles {
		out[i] = recentArticle{Title: a.Title, URL: a.URL, SourceName: a.SourceName, PublishedAt: formatPublishedAt(a.PublishedAt)}
	}
	return out
}

// formatPublishedAt turns an RFC3339 timestamp into a short display form
// (e.g. "Aug 13, 09:08"); articles are recent enough by construction that
// the year is rarely useful. Falls back to the raw value if it doesn't
// parse, rather than hiding a real (if unexpected) API response.
func formatPublishedAt(raw string) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return t.Format("Jan 2, 15:04")
}

// sourceBreakdownRow is the UI-side view of coreclient.SourceBreakdown,
// with BarPercent (0-100, relative to the entity's most-covering source)
// added for the proportional bar next to each row.
type sourceBreakdownRow struct {
	SourceName   string
	MentionCount int
	AvgSentiment float64
	BarPercent   int
}

func toSourceBreakdownRows(rows []coreclient.SourceBreakdown) []sourceBreakdownRow {
	max := 0
	for _, r := range rows {
		if r.MentionCount > max {
			max = r.MentionCount
		}
	}
	out := make([]sourceBreakdownRow, len(rows))
	for i, r := range rows {
		pct := 0
		if max > 0 {
			pct = r.MentionCount * 100 / max
		}
		out[i] = sourceBreakdownRow{SourceName: r.SourceName, MentionCount: r.MentionCount, AvgSentiment: r.AvgSentiment, BarPercent: pct}
	}
	return out
}

// rankedEntityRow is the UI-side view of coreclient.EntityRollup for the
// home page's per-type top-10 lists (issue #32/#33 revamp): Rank (1-based)
// and BarPercent (0-100, relative to the list's own top entity) replace
// the old table's redundant Type column — every row in a given list is
// already the same type, so showing it added nothing.
type rankedEntityRow struct {
	ID             int64
	Rank           int
	Name           string
	MentionCount   int
	SentimentScore float64
	BarPercent     int
}

func toRankedEntityRows(entities []coreclient.EntityRollup) []rankedEntityRow {
	max := 0
	for _, e := range entities {
		if e.MentionCount > max {
			max = e.MentionCount
		}
	}
	out := make([]rankedEntityRow, len(entities))
	for i, e := range entities {
		pct := 0
		if max > 0 {
			pct = e.MentionCount * 100 / max
		}
		out[i] = rankedEntityRow{ID: e.ID, Rank: i + 1, Name: e.Name, MentionCount: e.MentionCount, SentimentScore: e.SentimentScore, BarPercent: pct}
	}
	return out
}

// trendingChartPayload is marshaled to JSON and injected verbatim into the
// trending page's <script> block; json.Marshal HTML-escapes by default,
// so this is safe even though it's built from entity names we don't
// control.
type trendingChartPayload struct {
	TrendLabels       []string  `json:"trendLabels"`
	TrendMentions     []int     `json:"trendMentions"`
	TrendSentiment    []float64 `json:"trendSentiment"`
	SentimentPositive int       `json:"sentimentPositive"`
	SentimentNegative int       `json:"sentimentNegative"`
}

type trendingData struct {
	Windows          []windowTab
	WindowRangeLabel string
	ArticleCount     int
	EntityCount      int
	TopPersons       []rankedEntityRow
	TopOrgs          []rankedEntityRow
	TopTopics        []rankedEntityRow
	Sentiment        sentimentSummary
	RecentArticles   []recentArticle
	ChartDataJSON    template.JS
}

func buildTrendingData(window string, topPersons, topOrgs, topTopics []coreclient.EntityRollup, overall []coreclient.OverallTrendPoint, breakdown coreclient.SentimentBreakdown, windowStats coreclient.WindowStats, recentArticles []coreclient.RecentArticle) trendingData {
	trendLabels := make([]string, len(overall))
	trendMentions := make([]int, len(overall))
	trendSentiment := make([]float64, len(overall))
	for i, p := range overall {
		trendLabels[i] = p.WindowStart
		trendMentions[i] = p.TotalMentions
		trendSentiment[i] = p.AvgSentiment
	}

	payload := trendingChartPayload{
		TrendLabels: trendLabels, TrendMentions: trendMentions, TrendSentiment: trendSentiment,
		SentimentPositive: breakdown.Positive, SentimentNegative: breakdown.Negative,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ui: marshal trending chart data: %v", err)
		b = []byte("{}")
	}

	// The average is the current window's real aggregate sentiment
	// (already mention-count-weighted server-side), the same number the
	// time-series chart's most recent point shows — not derived from
	// only the top-N trending entities.
	var average float64
	if len(overall) > 0 {
		average = overall[len(overall)-1].AvgSentiment
	}

	return trendingData{
		Windows:          tabsFor(window),
		WindowRangeLabel: formatWindowRangeLabel(window, windowStats.WindowStart, windowStats.WindowEnd),
		ArticleCount:     windowStats.ArticleCount,
		EntityCount:      windowStats.EntityCount,
		TopPersons:       toRankedEntityRows(topPersons),
		TopOrgs:          toRankedEntityRows(topOrgs),
		TopTopics:        toRankedEntityRows(topTopics),
		Sentiment: sentimentSummary{
			Positive: breakdown.Positive, Negative: breakdown.Negative, Average: average,
		},
		RecentArticles: toRecentArticles(recentArticles),
		ChartDataJSON:  template.JS(b),
	}
}

// formatWindowRangeLabel turns a window's real start/end dates (as
// returned by core's /api/stats, "2006-01-02" formatted) into a short
// human label — e.g. "Aug 15, 2026" for a day, "Aug 10 – 16, 2026" for a
// week — so the dashboard shows exactly what date range is selected, not
// just a relative tab name. windowEnd is exclusive.
func formatWindowRangeLabel(window, windowStart, windowEnd string) string {
	start, err := time.Parse("2006-01-02", windowStart)
	if err != nil {
		return ""
	}
	end, err := time.Parse("2006-01-02", windowEnd)
	if err != nil {
		return ""
	}
	lastDay := end.AddDate(0, 0, -1)

	switch window {
	case "day":
		return start.Format("Mon, Jan 2, 2006")
	case "week":
		if start.Month() == lastDay.Month() {
			return fmt.Sprintf("%s %d–%d, %d", start.Format("Jan"), start.Day(), lastDay.Day(), start.Year())
		}
		return fmt.Sprintf("%s – %s", start.Format("Jan 2"), lastDay.Format("Jan 2, 2006"))
	case "month":
		return start.Format("January 2006")
	case "year":
		return start.Format("2006")
	default:
		return ""
	}
}

type entityChartPayload struct {
	Labels            []string  `json:"labels"`
	Mentions          []int     `json:"mentions"`
	Sentiment         []float64 `json:"sentiment"`
	SentimentPositive int       `json:"sentimentPositive"`
	SentimentNeutral  int       `json:"sentimentNeutral"`
	SentimentNegative int       `json:"sentimentNegative"`
}

type entityPageData struct {
	Detail          coreclient.EntityDetail
	SourceBreakdown []sourceBreakdownRow
	RecentArticles  []recentArticle
	RelatedEntities []coreclient.RelatedEntity
	ChartDataJSON   template.JS
}

func buildEntityPageData(detail coreclient.EntityDetail, sourceBreakdown []coreclient.SourceBreakdown, recentArticles []coreclient.RecentArticle, relatedEntities []coreclient.RelatedEntity) entityPageData {
	labels := make([]string, len(detail.Trend))
	mentions := make([]int, len(detail.Trend))
	sentiment := make([]float64, len(detail.Trend))
	for i, p := range detail.Trend {
		labels[i] = p.WindowStart
		mentions[i] = p.MentionCount
		sentiment[i] = p.SentimentScore
	}

	payload := entityChartPayload{Labels: labels, Mentions: mentions, Sentiment: sentiment}
	if n := len(detail.Trend); n > 0 {
		latest := detail.Trend[n-1]
		payload.SentimentPositive = latest.PositiveCount
		payload.SentimentNeutral = latest.NeutralCount
		payload.SentimentNegative = latest.NegativeCount
	}

	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ui: marshal entity chart data: %v", err)
		b = []byte("{}")
	}

	return entityPageData{
		Detail:          detail,
		SourceBreakdown: toSourceBreakdownRows(sourceBreakdown),
		RecentArticles:  toRecentArticles(recentArticles),
		RelatedEntities: relatedEntities,
		ChartDataJSON:   template.JS(b),
	}
}

type searchData struct {
	Query    string
	Searched bool
	Results  []coreclient.SearchResult
}

func renderPage(w io.Writer, name string, data any) {
	io.WriteString(w, layoutHeader)
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("ui: template error rendering %s: %v", name, err)
	}
	io.WriteString(w, layoutFooter)
}

func renderPartial(w io.Writer, name string, data any) {
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("ui: template error rendering %s: %v", name, err)
	}
}
