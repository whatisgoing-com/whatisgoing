package search

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// TestMeiliIndexer_Index runs against a real Meilisearch instance — set
// TEST_MEILISEARCH_URL (and TEST_MEILISEARCH_KEY if needed) to run it, e.g.
// against the docker-compose meilisearch service. Skipped otherwise.
func TestMeiliIndexer_IndexIsRetrievable(t *testing.T) {
	url := os.Getenv("TEST_MEILISEARCH_URL")
	if url == "" {
		t.Skip("TEST_MEILISEARCH_URL not set, skipping Meilisearch integration test")
	}
	apiKey := os.Getenv("TEST_MEILISEARCH_KEY")

	indexName := "articles_test"
	indexer := NewMeiliIndexer(url, apiKey, indexName)

	doc := Document{
		ID:          "smoke-test-1",
		URL:         "https://example.com/a",
		Title:       "Integration Test Article",
		Content:     "body",
		SourceID:    "src-1",
		PublishedAt: time.Now().UTC(),
	}

	if err := indexer.Index(context.Background(), doc); err != nil {
		t.Fatalf("Index() error = %v", err)
	}

	client := meilisearch.New(url, meilisearch.WithAPIKey(apiKey))
	idx := client.Index(indexName)

	var got Document
	deadline := time.Now().Add(5 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = idx.GetDocument(doc.ID, nil, &got)
		if lastErr == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("document not retrievable after indexing: %v", lastErr)
	}

	if got.Title != doc.Title {
		t.Errorf("got title %q, want %q", got.Title, doc.Title)
	}
}
