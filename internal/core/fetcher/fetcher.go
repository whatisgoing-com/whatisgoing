package fetcher

import (
	"context"
	"time"
)

type Source struct {
	ID   string
	Name string
	URL  string
}

type Article struct {
	SourceID    string
	URL         string
	Title       string
	Content     string
	PublishedAt time.Time
}

// Fetcher retrieves the current set of articles available from a source.
// Concrete implementations (RSS, HTML scraping) land in issue #2.
type Fetcher interface {
	Fetch(ctx context.Context, source Source) ([]Article, error)
}
