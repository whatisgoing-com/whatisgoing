package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverLinks_ResolvesRelativeAndFiltersCrossHost(t *testing.T) {
	page := `
		<html><body>
			<a href="/articles/one">One</a>
			<a href="/articles/two">Two</a>
			<a href="/articles/one">One again (duplicate)</a>
			<a href="https://other-site.example/away">Away</a>
			<a href="#top">Anchor only</a>
		</body></html>`

	links, err := discoverLinks(page, "https://news.example/section")
	if err != nil {
		t.Fatalf("discoverLinks() error = %v", err)
	}

	want := []string{
		"https://news.example/articles/one",
		"https://news.example/articles/two",
		"https://news.example/section",
	}
	if len(links) != len(want) {
		t.Fatalf("got %d links, want %d: %v", len(links), len(want), links)
	}
	for i, l := range links {
		if l != want[i] {
			t.Errorf("link[%d] = %q, want %q", i, l, want[i])
		}
	}
}

func TestExtractTitle(t *testing.T) {
	page := `<html><head><title>  Breaking News  </title></head><body></body></html>`
	if got := extractTitle(page); got != "Breaking News" {
		t.Errorf("extractTitle() = %q, want %q", got, "Breaking News")
	}
}

func TestHTMLFetcher_FetchDiscoversAndFetchesArticles(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/section", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
			<a href="/articles/one">One</a>
			<a href="/articles/two">Two</a>
		</body></html>`))
	})
	mux.HandleFunc("/articles/one", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Article One</title></head><body>body one</body></html>`))
	})
	mux.HandleFunc("/articles/two", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Article Two</title></head><body>body two</body></html>`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := NewHTMLFetcher(nil, 10)
	articles, err := f.Fetch(context.Background(), Source{ID: "src-1", URL: srv.URL + "/section"})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}
	if articles[0].Title != "Article One" || articles[1].Title != "Article Two" {
		t.Errorf("unexpected titles: %q, %q", articles[0].Title, articles[1].Title)
	}
	if articles[0].SourceID != "src-1" {
		t.Errorf("expected SourceID to be propagated, got %q", articles[0].SourceID)
	}
	if articles[0].DedupKey != articles[0].URL {
		t.Errorf("expected DedupKey to be the URL for HTML-sourced articles")
	}
}

func TestHTMLFetcher_RespectsMaxLinks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/section", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
			<a href="/a">a</a><a href="/b">b</a><a href="/c">c</a>
		</body></html>`))
	})
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html></html>")) })
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html></html>")) })
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<html></html>")) })

	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := NewHTMLFetcher(nil, 2)
	articles, err := f.Fetch(context.Background(), Source{ID: "src-1", URL: srv.URL + "/section"})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected MaxLinks=2 to cap results at 2, got %d", len(articles))
	}
}
