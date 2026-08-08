package fetcher

import (
	"context"
	"time"
)

type SourceType string

const (
	SourceTypeRSS  SourceType = "rss"
	SourceTypeHTML SourceType = "html"
)

type Source struct {
	ID   string
	Name string
	// URL is the RSS/Atom feed URL for SourceTypeRSS, or the listing page
	// to discover article links from for SourceTypeHTML.
	URL  string
	Type SourceType
}

type Article struct {
	SourceID string
	URL      string
	Title    string
	// Content holds the RSS item's body for SourceTypeRSS sources, or raw
	// HTML for SourceTypeHTML sources — cleaning/extraction happens in the
	// ner-sentiment service (issue #3), not here.
	Content     string
	PublishedAt time.Time
	// DedupKey identifies this article across fetches: the RSS GUID when
	// available, otherwise the canonical URL. Persistent dedup (across
	// runs) is enforced at the storage layer (issue #4) via a unique
	// constraint on this key; Fetcher implementations only need to avoid
	// returning duplicates within a single Fetch call.
	DedupKey string
}

// Fetcher retrieves the current set of articles available from a source.
type Fetcher interface {
	Fetch(ctx context.Context, source Source) ([]Article, error)
}
