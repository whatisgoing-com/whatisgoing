package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
	<title>Sample Feed</title>
	<item>
		<title>First Article</title>
		<link>https://example.com/first</link>
		<guid>guid-1</guid>
		<description>First article body</description>
		<pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate>
	</item>
	<item>
		<title>Duplicate GUID</title>
		<link>https://example.com/dup</link>
		<guid>guid-1</guid>
		<description>Should be filtered as a duplicate within this fetch</description>
	</item>
	<item>
		<title>Second Article</title>
		<link>https://example.com/second</link>
		<guid>guid-2</guid>
		<description>Second article body</description>
	</item>
</channel>
</rss>`

func TestRSSFetcher_ParsesItemsAndFiltersDuplicateGUIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(sampleFeed))
	}))
	defer srv.Close()

	f := NewRSSFetcher(nil)
	articles, err := f.Fetch(context.Background(), Source{ID: "src-1", URL: srv.URL})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("expected 2 articles (duplicate GUID filtered), got %d", len(articles))
	}

	if articles[0].DedupKey != "guid-1" || articles[0].Title != "First Article" {
		t.Errorf("unexpected first article: %+v", articles[0])
	}
	if articles[0].SourceID != "src-1" {
		t.Errorf("expected SourceID to be propagated, got %q", articles[0].SourceID)
	}
	if articles[1].DedupKey != "guid-2" {
		t.Errorf("unexpected second article: %+v", articles[1])
	}
}
