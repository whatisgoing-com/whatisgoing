package search

import (
	"context"
	"time"
)

type Document struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	SourceID    string    `json:"source_id"`
	PublishedAt time.Time `json:"published_at"`
}

// Indexer submits a document for search indexing. Meilisearch processes
// writes asynchronously, so a nil error here means the write was accepted,
// not that the document is already searchable.
type Indexer interface {
	Index(ctx context.Context, doc Document) error
}

// Searcher runs full-text queries against already-indexed documents.
// Separate from Indexer (write-side) so read-only callers, like the core
// API's search endpoint, don't need to depend on write capability.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]Document, error)
}
