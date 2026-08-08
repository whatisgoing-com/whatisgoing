package pipeline

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/ner"
)

// ArticleMentions pairs a fetched article with the entity mentions
// extracted from it by the ner-sentiment service (issue #3). Entities is
// nil if extraction failed for this article — the article is still saved,
// just without entity data.
type ArticleMentions struct {
	Article  fetcher.Article
	Entities []ner.Mention
}

// Store persists fetched articles and their extracted entity mentions.
type Store interface {
	SaveArticles(ctx context.Context, articles []ArticleMentions) error
}

// NERExtractor extracts entities and sentence-level sentiment from raw
// article content.
type NERExtractor interface {
	Extract(ctx context.Context, html, url string) (ner.ExtractResult, error)
}

const defaultNERConcurrency = 4

// Coordinator runs one fetch-extract-persist pass over a fixed set of
// sources.
type Coordinator struct {
	Fetcher fetcher.Fetcher
	NER     NERExtractor
	Store   Store
	Sources []fetcher.Source
	// NERConcurrency bounds how many ner-sentiment calls run at once.
	// Defaults to 4 if unset.
	NERConcurrency int
}

// Run fetches every source once, extracts entities/sentiment for each
// article, and persists the results. Persistent (cross-run) dedup is the
// storage layer's job (issue #4, via a unique constraint on
// Article.DedupKey) — this only filters duplicates that show up within a
// single Run, e.g. two configured sources happening to surface the same
// article.
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

		enriched := c.extractMentions(ctx, fresh)

		if err := c.Store.SaveArticles(ctx, enriched); err != nil {
			return fmt.Errorf("save articles from %s: %w", source.Name, err)
		}
	}

	return nil
}

// extractMentions calls the ner-sentiment service for each article through
// a bounded worker pool. A failure extracting one article is logged and
// that article is still returned (with no entities) rather than aborting
// the run.
func (c *Coordinator) extractMentions(ctx context.Context, articles []fetcher.Article) []ArticleMentions {
	concurrency := c.NERConcurrency
	if concurrency <= 0 {
		concurrency = defaultNERConcurrency
	}

	results := make([]ArticleMentions, len(articles))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, article := range articles {
		wg.Add(1)
		sem <- struct{}{}

		go func(i int, article fetcher.Article) {
			defer wg.Done()
			defer func() { <-sem }()

			result, err := c.NER.Extract(ctx, article.Content, article.URL)
			if err != nil {
				log.Printf("pipeline: ner extraction failed for %s (article still saved without entities): %v", article.URL, err)
				results[i] = ArticleMentions{Article: article}
				return
			}

			results[i] = ArticleMentions{Article: article, Entities: result.Entities}
		}(i, article)
	}

	wg.Wait()
	return results
}
