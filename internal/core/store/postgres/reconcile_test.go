package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/pipeline"
)

func TestStore_Reconcile_RepairsUnindexedArticles(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	failingIndexer := &fakeIndexer{err: errors.New("meilisearch down")}
	store := NewStore(pool, failingIndexer)
	seedSource(t, ctx, store)

	if err := store.SaveArticles(ctx, []pipeline.ArticleMentions{
		{Article: fetcher.Article{SourceID: "src-1", DedupKey: "dk-recon", URL: "https://example.com/r", Title: "R", Content: "body"}},
	}); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	// Outage is "fixed" — swap in a working indexer and reconcile.
	workingIndexer := &fakeIndexer{}
	store.indexer = workingIndexer

	repaired, err := store.Reconcile(ctx, 10)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if repaired != 1 {
		t.Fatalf("expected 1 repaired article, got %d", repaired)
	}
	if len(workingIndexer.docs) != 1 {
		t.Fatalf("expected Reconcile to call Index once, got %d", len(workingIndexer.docs))
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM articles WHERE dedup_key = 'dk-recon' AND indexed_at IS NOT NULL`).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected article marked indexed after reconcile, got count=%d", count)
	}
}

func TestStore_Reconcile_NoOpWhenNothingPending(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})

	repaired, err := store.Reconcile(context.Background(), 10)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if repaired != 0 {
		t.Errorf("expected 0 repaired with nothing pending, got %d", repaired)
	}
}
