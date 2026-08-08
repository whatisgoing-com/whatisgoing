package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/search"
)

// testPool runs against a real Postgres — set TEST_DATABASE_URL to run
// these tests, e.g. against the docker-compose postgres service:
// TEST_DATABASE_URL=postgres://whatisgoing:whatisgoing@localhost:5432/whatisgoing?sslmode=disable
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping Postgres integration test")
	}

	ctx := context.Background()

	if err := Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `TRUNCATE entity_cooccurrence, mentions, entities, articles, sources CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return pool
}

type fakeIndexer struct {
	docs []search.Document
	err  error
}

func (f *fakeIndexer) Index(ctx context.Context, doc search.Document) error {
	if f.err != nil {
		return f.err
	}
	f.docs = append(f.docs, doc)
	return nil
}

func seedSource(t *testing.T, ctx context.Context, store *Store) {
	t.Helper()
	err := store.UpsertSources(ctx, []fetcher.Source{
		{ID: "src-1", Name: "Src 1", URL: "https://example.com", Type: fetcher.SourceTypeRSS},
	})
	if err != nil {
		t.Fatalf("UpsertSources: %v", err)
	}
}

func TestStore_SaveArticles_InsertsAndIndexes(t *testing.T) {
	pool := testPool(t)
	indexer := &fakeIndexer{}
	store := NewStore(pool, indexer)
	ctx := context.Background()

	seedSource(t, ctx, store)

	articles := []fetcher.Article{
		{SourceID: "src-1", DedupKey: "dk-1", URL: "https://example.com/1", Title: "One", Content: "body one"},
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	if len(indexer.docs) != 1 {
		t.Fatalf("expected 1 indexed doc, got %d", len(indexer.docs))
	}
	if indexer.docs[0].Title != "One" {
		t.Errorf("unexpected indexed doc: %+v", indexer.docs[0])
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM articles WHERE dedup_key = 'dk-1' AND indexed_at IS NOT NULL`).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected article to be marked indexed_at, got count=%d", count)
	}
}

func TestStore_SaveArticles_DedupsAcrossRuns(t *testing.T) {
	pool := testPool(t)
	indexer := &fakeIndexer{}
	store := NewStore(pool, indexer)
	ctx := context.Background()

	seedSource(t, ctx, store)

	article := fetcher.Article{SourceID: "src-1", DedupKey: "dk-dup", URL: "https://example.com/dup", Title: "Dup", Content: "body"}

	if err := store.SaveArticles(ctx, []fetcher.Article{article}); err != nil {
		t.Fatalf("first SaveArticles: %v", err)
	}
	if err := store.SaveArticles(ctx, []fetcher.Article{article}); err != nil {
		t.Fatalf("second SaveArticles: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM articles WHERE dedup_key = 'dk-dup'`).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row after saving the same DedupKey twice, got %d", count)
	}
	if len(indexer.docs) != 1 {
		t.Errorf("expected Index to be called only once (not on the duplicate), got %d calls", len(indexer.docs))
	}
}

func TestStore_SaveArticles_LeavesIndexedAtNullOnIndexerFailure(t *testing.T) {
	pool := testPool(t)
	indexer := &fakeIndexer{err: errors.New("meilisearch down")}
	store := NewStore(pool, indexer)
	ctx := context.Background()

	seedSource(t, ctx, store)

	err := store.SaveArticles(ctx, []fetcher.Article{
		{SourceID: "src-1", DedupKey: "dk-fail", URL: "https://example.com/fail", Title: "Fail", Content: "body"},
	})
	if err != nil {
		t.Fatalf("SaveArticles should not fail when only the indexer fails: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM articles WHERE dedup_key = 'dk-fail' AND indexed_at IS NULL`).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected article saved with indexed_at NULL after indexer failure, got count=%d", count)
	}
}
