package main

import (
	"encoding/json"
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
	<script src="https://unpkg.com/htmx.org@2.0.3"></script>
	<script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
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

var tmpl = template.Must(template.New("ui").Parse(`
{{define "typeBadge"}}
{{if eq . "PERSON"}}<span class="inline-block rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">PERSON</span>
{{else if eq . "ORG"}}<span class="inline-block rounded-full bg-purple-50 px-2 py-0.5 text-xs font-medium text-purple-700">ORG</span>
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

<div class="flex gap-1 border-b border-gray-200">
	{{range .Windows}}<a href="/?window={{.Value}}" hx-get="/?window={{.Value}}" hx-target="#trending-panel" hx-push-url="true"
		class="-mb-px border-b-2 px-3 py-2 text-sm font-medium {{if .Active}}border-blue-600 text-blue-600{{else}}border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700{{end}}">{{.Label}}</a>{{end}}
</div>

<div id="trending-panel" class="space-y-6">
{{template "trendingPanel" .}}
</div>
{{end}}

{{define "trendingPanel"}}
<section class="grid grid-cols-2 gap-3 sm:grid-cols-4">
	<div class="rounded-xl border border-gray-200 bg-white p-4 text-center">
		<div class="text-2xl font-bold text-green-600">{{.Sentiment.Positive}}</div>
		<div class="text-xs font-medium uppercase tracking-wide text-gray-500">Positive</div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4 text-center">
		<div class="text-2xl font-bold text-gray-500">{{.Sentiment.Neutral}}</div>
		<div class="text-xs font-medium uppercase tracking-wide text-gray-500">Neutral</div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4 text-center">
		<div class="text-2xl font-bold text-red-600">{{.Sentiment.Negative}}</div>
		<div class="text-xs font-medium uppercase tracking-wide text-gray-500">Negative</div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4 text-center">
		<div class="text-2xl font-bold {{if gt .Sentiment.Average 0.0}}text-green-600{{else if lt .Sentiment.Average 0.0}}text-red-600{{else}}text-gray-500{{end}}">{{printf "%.2f" .Sentiment.Average}}</div>
		<div class="text-xs font-medium uppercase tracking-wide text-gray-500">Avg sentiment</div>
	</div>
</section>

<section class="grid grid-cols-1 gap-4 lg:grid-cols-3">
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-2 text-sm font-semibold text-gray-700">Top entities by mentions</h2>
		<div class="h-64"><canvas id="bar-chart"></canvas></div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-2 text-sm font-semibold text-gray-700">Mentions &amp; sentiment over time</h2>
		<div class="h-64"><canvas id="trend-chart"></canvas></div>
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-2 text-sm font-semibold text-gray-700">Overall sentiment breakdown</h2>
		<div class="h-64"><canvas id="sentiment-pie-chart"></canvas></div>
	</div>
</section>

<section class="rounded-xl border border-gray-200 bg-white p-4">
	<h2 class="mb-3 text-sm font-semibold text-gray-700">Entities</h2>
	{{template "entityList" .Entities}}
</section>

<section class="rounded-xl border border-gray-200 bg-white p-4">
	<h2 class="mb-1 text-sm font-semibold text-gray-700">Recent articles</h2>
	{{template "recentArticles" .RecentArticles}}
</section>

<script>
(function() {
	const chartData = {{.ChartDataJSON}};
	new Chart(document.getElementById('bar-chart'), {
		type: 'bar',
		data: { labels: chartData.labels, datasets: [{ label: 'Mentions', data: chartData.mentionCounts, backgroundColor: '#3b82f6', borderRadius: 4 }] },
		options: { maintainAspectRatio: false, indexAxis: 'y', plugins: { legend: { display: false } } }
	});
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
			labels: ['Positive', 'Neutral', 'Negative'],
			datasets: [{ data: [chartData.sentimentPositive, chartData.sentimentNeutral, chartData.sentimentNegative], backgroundColor: ['#16a34a', '#9ca3af', '#dc2626'] }]
		},
		options: { maintainAspectRatio: false }
	});
})();
</script>
{{end}}

{{define "entityList"}}
<div class="overflow-x-auto">
<table class="w-full text-sm">
	<thead>
		<tr class="border-b border-gray-200 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
			<th class="py-2 pr-4">Entity</th><th class="py-2 pr-4">Type</th><th class="py-2 pr-4 text-right">Mentions</th><th class="py-2 text-right">Sentiment</th>
		</tr>
	</thead>
	<tbody class="divide-y divide-gray-100">
	{{range .}}
		<tr class="hover:bg-gray-50">
			<td class="py-2 pr-4"><a href="/entities/{{.ID}}" class="font-medium text-gray-900 hover:text-blue-600">{{.Name}}</a></td>
			<td class="py-2 pr-4">{{template "typeBadge" .Type}}</td>
			<td class="py-2 pr-4 text-right tabular-nums">{{.MentionCount}}</td>
			<td class="py-2 text-right tabular-nums {{if gt .SentimentScore 0.0}}text-green-600{{else if lt .SentimentScore 0.0}}text-red-600{{else}}text-gray-500{{end}}">{{printf "%.2f" .SentimentScore}}</td>
		</tr>
	{{else}}
		<tr><td colspan="4" class="py-4 text-center text-gray-500">No trending entities yet.</td></tr>
	{{end}}
	</tbody>
</table>
</div>
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
			<span class="tabular-nums {{if gt .AvgSentiment 0.0}}text-green-600{{else if lt .AvgSentiment 0.0}}text-red-600{{else}}text-gray-500{{end}}">{{.MentionCount}} &middot; {{printf "%.2f" .AvgSentiment}}</span>
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

{{define "entityContent"}}
<p><a href="/" class="text-sm text-blue-600 hover:underline">&larr; Trending</a></p>
<div class="flex items-center gap-2">
	<h1 class="text-2xl font-bold tracking-tight">{{.Detail.Name}}</h1>
	{{template "typeBadge" .Detail.Type}}
</div>

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

<section class="grid grid-cols-1 gap-4 lg:grid-cols-2">
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-3 text-sm font-semibold text-gray-700">By source</h2>
		{{template "sourceBreakdown" .SourceBreakdown}}
	</div>
	<div class="rounded-xl border border-gray-200 bg-white p-4">
		<h2 class="mb-1 text-sm font-semibold text-gray-700">Recent articles</h2>
		{{template "recentArticles" .RecentArticles}}
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
			<td class="py-2 text-right tabular-nums {{if gt .SentimentScore 0.0}}text-green-600{{else if lt .SentimentScore 0.0}}text-red-600{{else}}text-gray-500{{end}}">{{printf "%.2f" .SentimentScore}}</td>
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

func tabsFor(active string) []windowTab {
	tabs := make([]windowTab, 0, len(windowTabs))
	for _, w := range windowTabs {
		tabs = append(tabs, windowTab{Value: w.Value, Label: w.Label, Active: w.Value == active})
	}
	return tabs
}

type sentimentSummary struct {
	Positive int
	Neutral  int
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

// trendingChartPayload is marshaled to JSON and injected verbatim into the
// trending page's <script> block; json.Marshal HTML-escapes by default,
// so this is safe even though it's built from entity names we don't
// control.
type trendingChartPayload struct {
	Labels            []string  `json:"labels"`
	MentionCounts     []int     `json:"mentionCounts"`
	TrendLabels       []string  `json:"trendLabels"`
	TrendMentions     []int     `json:"trendMentions"`
	TrendSentiment    []float64 `json:"trendSentiment"`
	SentimentPositive int       `json:"sentimentPositive"`
	SentimentNeutral  int       `json:"sentimentNeutral"`
	SentimentNegative int       `json:"sentimentNegative"`
}

type trendingData struct {
	Windows        []windowTab
	Entities       []coreclient.EntityRollup
	Sentiment      sentimentSummary
	RecentArticles []recentArticle
	ChartDataJSON  template.JS
}

func buildTrendingData(window string, entities []coreclient.EntityRollup, overall []coreclient.OverallTrendPoint, breakdown coreclient.SentimentBreakdown, recentArticles []coreclient.RecentArticle) trendingData {
	labels := make([]string, len(entities))
	counts := make([]int, len(entities))
	for i, e := range entities {
		labels[i] = e.Name
		counts[i] = e.MentionCount
	}

	trendLabels := make([]string, len(overall))
	trendMentions := make([]int, len(overall))
	trendSentiment := make([]float64, len(overall))
	for i, p := range overall {
		trendLabels[i] = p.WindowStart
		trendMentions[i] = p.TotalMentions
		trendSentiment[i] = p.AvgSentiment
	}

	payload := trendingChartPayload{
		Labels: labels, MentionCounts: counts,
		TrendLabels: trendLabels, TrendMentions: trendMentions, TrendSentiment: trendSentiment,
		SentimentPositive: breakdown.Positive, SentimentNeutral: breakdown.Neutral, SentimentNegative: breakdown.Negative,
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
		Windows:  tabsFor(window),
		Entities: entities,
		Sentiment: sentimentSummary{
			Positive: breakdown.Positive, Neutral: breakdown.Neutral, Negative: breakdown.Negative, Average: average,
		},
		RecentArticles: toRecentArticles(recentArticles),
		ChartDataJSON:  template.JS(b),
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
	ChartDataJSON   template.JS
}

func buildEntityPageData(detail coreclient.EntityDetail, sourceBreakdown []coreclient.SourceBreakdown, recentArticles []coreclient.RecentArticle) entityPageData {
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
