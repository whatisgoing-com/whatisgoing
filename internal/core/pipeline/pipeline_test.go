package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/ner"
)

type fakeFetcher struct {
	articles []fetcher.Article
	err      error
}

func (f *fakeFetcher) Fetch(ctx context.Context, source fetcher.Source) ([]fetcher.Article, error) {
	return f.articles, f.err
}

type fakeNER struct {
	result ner.ExtractResult
	err    error
	// failFor, if set, only fails for articles with this URL — used to
	// test partial-failure handling without failing every article.
	failFor string
}

func (n *fakeNER) Extract(ctx context.Context, html, url string) (ner.ExtractResult, error) {
	if n.err != nil && (n.failFor == "" || n.failFor == url) {
		return ner.ExtractResult{}, n.err
	}
	return n.result, nil
}

type fakeStore struct {
	saved []ArticleMentions
	err   error
}

func (s *fakeStore) SaveArticles(ctx context.Context, articles []ArticleMentions) error {
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, articles...)
	return nil
}

func TestCoordinatorRun_SavesFetchedArticlesWithEntities(t *testing.T) {
	f := &fakeFetcher{articles: []fetcher.Article{{Title: "hello", URL: "https://a"}}}
	n := &fakeNER{result: ner.ExtractResult{Entities: []ner.Mention{{Text: "Someone", Type: "PERSON"}}}}
	s := &fakeStore{}
	c := &Coordinator{
		Fetcher: f,
		NER:     n,
		Store:   s,
		Sources: []fetcher.Source{{Name: "src-a"}, {Name: "src-b"}},
	}

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(s.saved) != 2 {
		t.Fatalf("expected 2 saved articles (one per source), got %d", len(s.saved))
	}
	for _, am := range s.saved {
		if len(am.Entities) != 1 || am.Entities[0].Text != "Someone" {
			t.Errorf("expected entities to be attached, got %+v", am)
		}
	}
}

func TestCoordinatorRun_FiltersDuplicateDedupKeysAcrossSources(t *testing.T) {
	f := &fakeFetcher{articles: []fetcher.Article{{Title: "same article", DedupKey: "shared-key"}}}
	s := &fakeStore{}
	c := &Coordinator{
		Fetcher: f,
		NER:     &fakeNER{},
		Store:   s,
		// Two sources whose fetches happen to surface the same DedupKey.
		Sources: []fetcher.Source{{Name: "src-a"}, {Name: "src-b"}},
	}

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(s.saved) != 1 {
		t.Fatalf("expected duplicate DedupKey to be filtered, saved %d articles", len(s.saved))
	}
}

func TestCoordinatorRun_ReturnsFetchError(t *testing.T) {
	f := &fakeFetcher{err: errors.New("boom")}
	c := &Coordinator{
		Fetcher: f,
		NER:     &fakeNER{},
		Store:   &fakeStore{},
		Sources: []fetcher.Source{{Name: "src-a"}},
	}

	if err := c.Run(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCoordinatorRun_ReturnsStoreError(t *testing.T) {
	f := &fakeFetcher{articles: []fetcher.Article{{Title: "hello"}}}
	s := &fakeStore{err: errors.New("boom")}
	c := &Coordinator{
		Fetcher: f,
		NER:     &fakeNER{},
		Store:   s,
		Sources: []fetcher.Source{{Name: "src-a"}},
	}

	if err := c.Run(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCoordinatorRun_NERFailureForOneArticleDoesNotAbortRun(t *testing.T) {
	f := &fakeFetcher{articles: []fetcher.Article{
		{Title: "ok", URL: "https://ok"},
		{Title: "bad", URL: "https://bad"},
	}}
	n := &fakeNER{
		err:     errors.New("ner-sentiment down"),
		failFor: "https://bad",
		result:  ner.ExtractResult{Entities: []ner.Mention{{Text: "Someone", Type: "PERSON"}}},
	}
	s := &fakeStore{}
	c := &Coordinator{
		Fetcher: f,
		NER:     n,
		Store:   s,
		Sources: []fetcher.Source{{Name: "src-a"}},
	}

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v, want nil (a single NER failure should not abort the run)", err)
	}

	if len(s.saved) != 2 {
		t.Fatalf("expected both articles saved, got %d", len(s.saved))
	}

	var ok, bad ArticleMentions
	for _, am := range s.saved {
		if am.Article.URL == "https://ok" {
			ok = am
		} else {
			bad = am
		}
	}
	if len(ok.Entities) != 1 {
		t.Errorf("expected the successful article to have entities, got %+v", ok)
	}
	if len(bad.Entities) != 0 {
		t.Errorf("expected the failed article to have no entities, got %+v", bad)
	}
}
