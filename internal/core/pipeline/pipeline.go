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

func (c *Coordinator) Run(ctx context.Context) error {
	for _, source := range c.Sources {
		articles, err := c.Fetcher.Fetch(ctx, source)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", source.Name, err)
		}

		if err := c.Store.SaveArticles(ctx, articles); err != nil {
			return fmt.Errorf("save articles from %s: %w", source.Name, err)
		}
	}

	return nil
}
