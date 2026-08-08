package main

import (
	"encoding/json"
	"html/template"
	"io"
	"log"

	"github.com/whatisgoing-com/whatisgoing/internal/ui/coreclient"
)

const layoutHeader = `<!doctype html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<title>whatisgoing.com</title>
	<script src="https://unpkg.com/htmx.org@2.0.3"></script>
	<script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
	<style>
		body { font-family: system-ui, sans-serif; max-width: 820px; margin: 2rem auto; padding: 0 1rem; color: #1a1a1a; }
		nav a { margin-right: 1rem; }
		table { border-collapse: collapse; width: 100%; }
		th, td { text-align: left; padding: 0.4rem 0.6rem; border-bottom: 1px solid #ddd; }
		.tabs a { margin-right: 0.75rem; text-decoration: none; }
		.tabs a.active { font-weight: bold; text-decoration: underline; }
		.sentiment-positive { color: #1a7f37; }
		.sentiment-negative { color: #b91c1c; }
		ul { list-style: none; padding: 0; }
		li { padding: 0.4rem 0; border-bottom: 1px solid #eee; }
		.sentiment-overview { background: #f6f8fa; border-radius: 8px; padding: 0.75rem 1rem; margin: 1rem 0; }
		.charts { display: flex; gap: 1.5rem; flex-wrap: wrap; margin: 1rem 0; }
		.chart-box { flex: 1 1 320px; height: 260px; }
		.entity-search { position: relative; margin: 1rem 0; }
		.entity-search input { width: 100%; padding: 0.5rem; box-sizing: border-box; }
		.entity-search-results { position: absolute; z-index: 10; background: #fff; border: 1px solid #ddd; width: 100%; box-sizing: border-box; }
		.entity-search-results li { padding: 0.4rem 0.6rem; }
	</style>
</head>
<body>
	<nav>
		<a href="/">Trending</a>
		<a href="/search">Search</a>
	</nav>
	<main>
`

const layoutFooter = `
	</main>
</body>
</html>
`

var tmpl = template.Must(template.New("ui").Parse(`
{{define "trendingContent"}}
<h1>Trending</h1>

<div class="entity-search">
	<input type="search" placeholder="Find an entity&hellip;" autocomplete="off"
		hx-get="/entities/search" hx-trigger="keyup changed delay:300ms, search" hx-target="#entity-search-results" name="q">
	<div id="entity-search-results"></div>
</div>

<div class="tabs">
	{{range .Windows}}<a href="/?window={{.Value}}" hx-get="/?window={{.Value}}" hx-target="#trending-panel" hx-push-url="true" class="{{if .Active}}active{{end}}">{{.Label}}</a>{{end}}
</div>
<div id="trending-panel">
{{template "trendingPanel" .}}
</div>
{{end}}

{{define "trendingPanel"}}
<section class="sentiment-overview">
	<strong>Sentiment overview:</strong>
	{{.Sentiment.Positive}} positive &middot; {{.Sentiment.Neutral}} neutral &middot; {{.Sentiment.Negative}} negative
	&mdash; average {{printf "%.2f" .Sentiment.Average}}
</section>

<div class="charts">
	<div class="chart-box"><canvas id="bar-chart"></canvas></div>
	<div class="chart-box"><canvas id="trend-chart"></canvas></div>
</div>

{{template "entityList" .Entities}}

<script>
(function() {
	const chartData = {{.ChartDataJSON}};
	new Chart(document.getElementById('bar-chart'), {
		type: 'bar',
		data: { labels: chartData.labels, datasets: [{ label: 'Mentions', data: chartData.mentionCounts, backgroundColor: '#3b82f6' }] },
		options: { maintainAspectRatio: false, indexAxis: 'y', plugins: { legend: { display: false }, title: { display: true, text: 'Top entities by mentions' } } }
	});
	new Chart(document.getElementById('trend-chart'), {
		type: 'line',
		data: {
			labels: chartData.trendLabels,
			datasets: [
				{ label: 'Mentions', data: chartData.trendMentions, borderColor: '#3b82f6', yAxisID: 'y' },
				{ label: 'Avg sentiment', data: chartData.trendSentiment, borderColor: '#1a7f37', yAxisID: 'y1' }
			]
		},
		options: {
			maintainAspectRatio: false,
			plugins: { title: { display: true, text: 'Mentions & sentiment over time' } },
			scales: {
				y: { type: 'linear', position: 'left', beginAtZero: true },
				y1: { type: 'linear', position: 'right', min: -1, max: 1, grid: { drawOnChartArea: false } }
			}
		}
	});
})();
</script>
{{end}}

{{define "entityList"}}
<table>
	<thead><tr><th>Entity</th><th>Type</th><th>Mentions</th><th>Sentiment</th></tr></thead>
	<tbody>
	{{range .}}
		<tr>
			<td><a href="/entities/{{.ID}}">{{.Name}}</a></td>
			<td>{{.Type}}</td>
			<td>{{.MentionCount}}</td>
			<td class="{{if gt .SentimentScore 0.0}}sentiment-positive{{else if lt .SentimentScore 0.0}}sentiment-negative{{end}}">{{printf "%.2f" .SentimentScore}}</td>
		</tr>
	{{else}}
		<tr><td colspan="4">No trending entities yet.</td></tr>
	{{end}}
	</tbody>
</table>
{{end}}

{{define "entitySearchResults"}}
{{if .}}
<ul class="entity-search-results">
	{{range .}}
		<li><a href="/entities/{{.ID}}">{{.Name}}</a> <small>({{.Type}})</small></li>
	{{end}}
</ul>
{{end}}
{{end}}

{{define "entityContent"}}
<p><a href="/">&larr; Trending</a></p>
<h1>{{.Detail.Name}} <small>({{.Detail.Type}})</small></h1>
<div class="chart-box"><canvas id="entity-trend-chart"></canvas></div>
<table>
	<thead><tr><th>Date</th><th>Mentions</th><th>Sentiment</th></tr></thead>
	<tbody>
	{{range .Detail.Trend}}
		<tr>
			<td>{{.WindowStart}}</td>
			<td>{{.MentionCount}}</td>
			<td class="{{if gt .SentimentScore 0.0}}sentiment-positive{{else if lt .SentimentScore 0.0}}sentiment-negative{{end}}">{{printf "%.2f" .SentimentScore}}</td>
		</tr>
	{{end}}
	</tbody>
</table>
<script>
(function() {
	const chartData = {{.ChartDataJSON}};
	new Chart(document.getElementById('entity-trend-chart'), {
		type: 'line',
		data: {
			labels: chartData.labels,
			datasets: [
				{ label: 'Mentions', data: chartData.mentions, borderColor: '#3b82f6', yAxisID: 'y' },
				{ label: 'Sentiment', data: chartData.sentiment, borderColor: '#1a7f37', yAxisID: 'y1' }
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
})();
</script>
{{end}}

{{define "entityNotFound"}}
<p><a href="/">&larr; Trending</a></p>
<h1>Not found</h1>
<p>No such entity, or it hasn't been through a rollup yet.</p>
{{end}}

{{define "searchContent"}}
<h1>Search</h1>
<form hx-get="/search" hx-target="#search-results" hx-push-url="true">
	<input type="search" name="q" value="{{.Query}}" placeholder="Search articles&hellip;" autofocus>
	<button type="submit">Search</button>
</form>
<div id="search-results">
{{template "searchResults" .}}
</div>
{{end}}

{{define "searchResults"}}
{{if .Searched}}
<ul>
	{{range .Results}}
		<li><a href="{{.URL}}" target="_blank" rel="noopener">{{.Title}}</a> &mdash; {{.PublishedAt}}</li>
	{{else}}
		<li>No results.</li>
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

func summarizeSentiment(entities []coreclient.EntityRollup) sentimentSummary {
	var s sentimentSummary
	var total float64
	for _, e := range entities {
		switch {
		case e.SentimentScore > 0:
			s.Positive++
		case e.SentimentScore < 0:
			s.Negative++
		default:
			s.Neutral++
		}
		total += e.SentimentScore
	}
	if len(entities) > 0 {
		s.Average = total / float64(len(entities))
	}
	return s
}

// trendingChartPayload is marshaled to JSON and injected verbatim into the
// trending page's <script> block; json.Marshal HTML-escapes by default,
// so this is safe even though it's built from entity names we don't
// control.
type trendingChartPayload struct {
	Labels         []string  `json:"labels"`
	MentionCounts  []int     `json:"mentionCounts"`
	TrendLabels    []string  `json:"trendLabels"`
	TrendMentions  []int     `json:"trendMentions"`
	TrendSentiment []float64 `json:"trendSentiment"`
}

type trendingData struct {
	Windows       []windowTab
	Entities      []coreclient.EntityRollup
	Sentiment     sentimentSummary
	ChartDataJSON template.JS
}

func buildTrendingData(window string, entities []coreclient.EntityRollup, overall []coreclient.OverallTrendPoint) trendingData {
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
	}
	b, err := json.Marshal(payload)
	if err != nil {
		log.Printf("ui: marshal trending chart data: %v", err)
		b = []byte("{}")
	}

	return trendingData{
		Windows:       tabsFor(window),
		Entities:      entities,
		Sentiment:     summarizeSentiment(entities),
		ChartDataJSON: template.JS(b),
	}
}

type entityChartPayload struct {
	Labels    []string  `json:"labels"`
	Mentions  []int     `json:"mentions"`
	Sentiment []float64 `json:"sentiment"`
}

type entityPageData struct {
	Detail        coreclient.EntityDetail
	ChartDataJSON template.JS
}

func buildEntityPageData(detail coreclient.EntityDetail) entityPageData {
	labels := make([]string, len(detail.Trend))
	mentions := make([]int, len(detail.Trend))
	sentiment := make([]float64, len(detail.Trend))
	for i, p := range detail.Trend {
		labels[i] = p.WindowStart
		mentions[i] = p.MentionCount
		sentiment[i] = p.SentimentScore
	}

	b, err := json.Marshal(entityChartPayload{Labels: labels, Mentions: mentions, Sentiment: sentiment})
	if err != nil {
		log.Printf("ui: marshal entity chart data: %v", err)
		b = []byte("{}")
	}

	return entityPageData{Detail: detail, ChartDataJSON: template.JS(b)}
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
