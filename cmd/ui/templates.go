package main

import (
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
	<style>
		body { font-family: system-ui, sans-serif; max-width: 760px; margin: 2rem auto; padding: 0 1rem; color: #1a1a1a; }
		nav a { margin-right: 1rem; }
		table { border-collapse: collapse; width: 100%; }
		th, td { text-align: left; padding: 0.4rem 0.6rem; border-bottom: 1px solid #ddd; }
		.tabs a { margin-right: 0.75rem; text-decoration: none; }
		.tabs a.active { font-weight: bold; text-decoration: underline; }
		.sentiment-positive { color: #1a7f37; }
		.sentiment-negative { color: #b91c1c; }
		ul { list-style: none; padding: 0; }
		li { padding: 0.4rem 0; border-bottom: 1px solid #eee; }
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
<div class="tabs">
	{{range .Windows}}<a href="/?window={{.Value}}" hx-get="/?window={{.Value}}" hx-target="#trending-list" hx-push-url="true" class="{{if .Active}}active{{end}}">{{.Label}}</a>{{end}}
</div>
<div id="trending-list">
{{template "entityList" .Entities}}
</div>
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

{{define "entityContent"}}
<p><a href="/">&larr; Trending</a></p>
<h1>{{.Name}} <small>({{.Type}})</small></h1>
<table>
	<thead><tr><th>Date</th><th>Mentions</th><th>Sentiment</th></tr></thead>
	<tbody>
	{{range .Trend}}
		<tr>
			<td>{{.WindowStart}}</td>
			<td>{{.MentionCount}}</td>
			<td class="{{if gt .SentimentScore 0.0}}sentiment-positive{{else if lt .SentimentScore 0.0}}sentiment-negative{{end}}">{{printf "%.2f" .SentimentScore}}</td>
		</tr>
	{{end}}
	</tbody>
</table>
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

type trendingData struct {
	Windows  []windowTab
	Entities []coreclient.EntityRollup
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
