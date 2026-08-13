package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/ner"
	"github.com/whatisgoing-com/whatisgoing/internal/core/pipeline"
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

	if _, err := pool.Exec(ctx, `TRUNCATE entity_rollups, entity_cooccurrence, mentions, entities, articles, sources CASCADE`); err != nil {
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

	articles := []pipeline.ArticleMentions{
		{Article: fetcher.Article{SourceID: "src-1", DedupKey: "dk-1", URL: "https://example.com/1", Title: "One", Content: "body one"}},
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

	if err := store.SaveArticles(ctx, []pipeline.ArticleMentions{{Article: article}}); err != nil {
		t.Fatalf("first SaveArticles: %v", err)
	}
	if err := store.SaveArticles(ctx, []pipeline.ArticleMentions{{Article: article}}); err != nil {
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

	err := store.SaveArticles(ctx, []pipeline.ArticleMentions{
		{Article: fetcher.Article{SourceID: "src-1", DedupKey: "dk-fail", URL: "https://example.com/fail", Title: "Fail", Content: "body"}},
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

func TestStore_SaveArticles_PersistsAggregatedMentionsAndCooccurrence(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	ctx := context.Background()

	seedSource(t, ctx, store)

	article := fetcher.Article{SourceID: "src-1", DedupKey: "dk-entities", URL: "https://example.com/e", Title: "E", Content: "body"}
	entities := []ner.Mention{
		{Text: "Elon Musk", Type: "PERSON", SentimentScore: 0.8},
		{Text: "Elon Musk", Type: "PERSON", SentimentScore: 0.4},
		{Text: "Tesla", Type: "ORG", SentimentScore: -0.2},
	}

	if err := store.SaveArticles(ctx, []pipeline.ArticleMentions{{Article: article, Entities: entities}}); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	var articleID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM articles WHERE dedup_key = 'dk-entities'`).Scan(&articleID); err != nil {
		t.Fatalf("query article id: %v", err)
	}

	var muskID, teslaID int64
	var muskCount int
	var muskSentiment float64
	if err := pool.QueryRow(ctx, `
		SELECT e.id, m.mention_count, m.sentiment_score
		FROM entities e JOIN mentions m ON m.entity_id = e.id
		WHERE e.name = 'Elon Musk' AND e.type = 'PERSON' AND m.article_id = $1`, articleID,
	).Scan(&muskID, &muskCount, &muskSentiment); err != nil {
		t.Fatalf("query Elon Musk mention: %v", err)
	}
	if muskCount != 1 {
		t.Errorf("expected mention_count=1 for Elon Musk (2 occurrences within 1 article still count as 1 mention), got %d", muskCount)
	}
	if want := 0.6; muskSentiment < want-0.001 || muskSentiment > want+0.001 {
		t.Errorf("expected averaged sentiment_score=%v for Elon Musk, got %v", want, muskSentiment)
	}

	if err := pool.QueryRow(ctx, `SELECT id FROM entities WHERE name = 'Tesla' AND type = 'ORG'`).Scan(&teslaID); err != nil {
		t.Fatalf("query Tesla entity: %v", err)
	}

	var cooccurrenceCount int
	a, b := muskID, teslaID
	if a > b {
		a, b = b, a
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM entity_cooccurrence
		WHERE article_id = $1 AND entity_a_id = $2 AND entity_b_id = $3`, articleID, a, b,
	).Scan(&cooccurrenceCount); err != nil {
		t.Fatalf("query cooccurrence: %v", err)
	}
	if cooccurrenceCount != 1 {
		t.Errorf("expected 1 cooccurrence row for (Elon Musk, Tesla), got %d", cooccurrenceCount)
	}
}

func TestStore_SaveArticles_SharedEntityAcrossArticlesAccumulatesSeparateMentions(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	ctx := context.Background()

	seedSource(t, ctx, store)

	first := pipeline.ArticleMentions{
		Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-shared-1", URL: "https://example.com/s1", Title: "S1", Content: "body"},
		Entities: []ner.Mention{{Text: "Apple", Type: "ORG", SentimentScore: 0.5}},
	}
	second := pipeline.ArticleMentions{
		Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-shared-2", URL: "https://example.com/s2", Title: "S2", Content: "body"},
		Entities: []ner.Mention{{Text: "Apple", Type: "ORG", SentimentScore: -0.5}},
	}

	if err := store.SaveArticles(ctx, []pipeline.ArticleMentions{first, second}); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	var entityCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM entities WHERE name = 'Apple' AND type = 'ORG'`).Scan(&entityCount); err != nil {
		t.Fatalf("query entity count: %v", err)
	}
	if entityCount != 1 {
		t.Errorf("expected the Apple entity to be shared (deduped) across articles, got %d rows", entityCount)
	}

	var mentionRows int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM mentions m
		JOIN entities e ON e.id = m.entity_id
		WHERE e.name = 'Apple' AND e.type = 'ORG'`,
	).Scan(&mentionRows); err != nil {
		t.Fatalf("query mention rows: %v", err)
	}
	if mentionRows != 2 {
		t.Errorf("expected 2 separate mentions rows (one per article), got %d", mentionRows)
	}
}

func TestStore_SaveArticles_NormalizesPossessiveEntityNames(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	ctx := context.Background()

	seedSource(t, ctx, store)

	first := pipeline.ArticleMentions{
		Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-poss-1", URL: "https://example.com/p1", Title: "P1", Content: "body"},
		Entities: []ner.Mention{{Text: "Donald Trump's", Type: "PERSON", SentimentScore: -0.2}},
	}
	second := pipeline.ArticleMentions{
		Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-poss-2", URL: "https://example.com/p2", Title: "P2", Content: "body"},
		Entities: []ner.Mention{{Text: "Donald Trump", Type: "PERSON", SentimentScore: 0.4}},
	}

	if err := store.SaveArticles(ctx, []pipeline.ArticleMentions{first, second}); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM entities WHERE type = 'PERSON' AND name ILIKE 'Donald Trump%'`).Scan(&count); err != nil {
		t.Fatalf("query entity count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 'Donald Trump's' and 'Donald Trump' to collapse into 1 entity, got %d", count)
	}

	var name string
	if err := pool.QueryRow(ctx, `SELECT name FROM entities WHERE type = 'PERSON' AND name ILIKE 'Donald Trump%'`).Scan(&name); err != nil {
		t.Fatalf("query entity name: %v", err)
	}
	if name != "Donald Trump" {
		t.Errorf("expected stored name to be the possessive-stripped form 'Donald Trump', got %q", name)
	}
}
