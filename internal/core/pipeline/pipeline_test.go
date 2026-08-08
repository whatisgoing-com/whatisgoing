package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
)

type fakeFetcher struct {
	articles []fetcher.Article
	err      error
}

func (f *fakeFetcher) Fetch(ctx context.Context, source fetcher.Source) ([]fetcher.Article, error) {
	return f.articles, f.err
}

type fakeStore struct {
	saved []fetcher.Article
	err   error
}

func (s *fakeStore) SaveArticles(ctx context.Context, articles []fetcher.Article) error {
	if s.err != nil {
		return s.err
	}
	s.saved = append(s.saved, articles...)
	return nil
}

func TestCoordinatorRun_SavesFetchedArticles(t *testing.T) {
	f := &fakeFetcher{articles: []fetcher.Article{{Title: "hello"}}}
	s := &fakeStore{}
	c := &Coordinator{
		Fetcher: f,
		Store:   s,
		Sources: []fetcher.Source{{Name: "src-a"}, {Name: "src-b"}},
	}

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(s.saved) != 2 {
		t.Fatalf("expected 2 saved articles (one per source), got %d", len(s.saved))
	}
}

func TestCoordinatorRun_FiltersDuplicateDedupKeysAcrossSources(t *testing.T) {
	f := &fakeFetcher{articles: []fetcher.Article{{Title: "same article", DedupKey: "shared-key"}}}
	s := &fakeStore{}
	c := &Coordinator{
		Fetcher: f,
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
		Store:   s,
		Sources: []fetcher.Source{{Name: "src-a"}},
	}

	if err := c.Run(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
}
