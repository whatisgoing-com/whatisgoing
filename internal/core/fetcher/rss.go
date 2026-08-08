package fetcher

import (
	"context"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

// RSSFetcher fetches and parses an RSS/Atom feed.
type RSSFetcher struct {
	Client *http.Client
}

func NewRSSFetcher(client *http.Client) *RSSFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	return &RSSFetcher{Client: client}
}

func (f *RSSFetcher) Fetch(ctx context.Context, source Source) ([]Article, error) {
	parser := gofeed.NewParser()
	parser.Client = f.Client

	feed, err := parser.ParseURLWithContext(source.URL, ctx)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(feed.Items))
	articles := make([]Article, 0, len(feed.Items))

	for _, item := range feed.Items {
		dedupKey := item.GUID
		if dedupKey == "" {
			dedupKey = item.Link
		}
		if dedupKey == "" || seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true

		var published time.Time
		if item.PublishedParsed != nil {
			published = *item.PublishedParsed
		}

		content := item.Content
		if content == "" {
			content = item.Description
		}

		articles = append(articles, Article{
			SourceID:    source.ID,
			URL:         item.Link,
			Title:       item.Title,
			Content:     content,
			PublishedAt: published,
			DedupKey:    dedupKey,
		})
	}

	return articles, nil
}
