package pipeline

import (
	"context"
	"fmt"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
)

// Store persists fetched articles. The Postgres/Meilisearch-backed
// implementation lands in issue #4.
type Store interface {
	SaveArticles(ctx context.Context, articles []fetcher.Article) error
}

// Coordinator runs one fetch-and-persist pass over a fixed set of sources.
type Coordinator struct {
	Fetcher fetcher.Fetcher
	Store   Store
	Sources []fetcher.Source
}

// Run fetches every source once and persists the results. Persistent
// (cross-run) dedup is the storage layer's job (issue #4, via a unique
// constraint on Article.DedupKey) — this only filters duplicates that show
// up within a single Run, e.g. two configured sources happening to surface
// the same article.
func (c *Coordinator) Run(ctx context.Context) error {
	seen := make(map[string]bool)

	for _, source := range c.Sources {
		articles, err := c.Fetcher.Fetch(ctx, source)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", source.Name, err)
		}

		fresh := make([]fetcher.Article, 0, len(articles))
		for _, article := range articles {
			if article.DedupKey != "" && seen[article.DedupKey] {
				continue
			}
			seen[article.DedupKey] = true
			fresh = append(fresh, article)
		}

		if err := c.Store.SaveArticles(ctx, fresh); err != nil {
			return fmt.Errorf("save articles from %s: %w", source.Name, err)
		}
	}

	return nil
}
