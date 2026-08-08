package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/ner"
	"github.com/whatisgoing-com/whatisgoing/internal/core/pipeline"
	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
)

func TestRollupStore_Compute_AggregatesMentionCountAndAveragesSentimentPerDay(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	seedSource(t, ctx, store)

	published := time.Date(2026, time.August, 8, 9, 0, 0, 0, time.UTC)

	articles := []pipeline.ArticleMentions{
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-r1", URL: "https://example.com/r1", Title: "R1", Content: "body", PublishedAt: published},
			Entities: []ner.Mention{{Text: "Elon Musk", Type: "PERSON", SentimentScore: 0.8}},
		},
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-r2", URL: "https://example.com/r2", Title: "R2", Content: "body", PublishedAt: published.Add(2 * time.Hour)},
			Entities: []ner.Mention{{Text: "Elon Musk", Type: "PERSON", SentimentScore: 0.4}},
		},
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	if err := rollupStore.Compute(ctx); err != nil {
		t.Fatalf("Compute: %v", err)
	}

	dayStart := rollup.WindowStart(rollup.Day, published)
	results, err := rollupStore.TopEntities(ctx, rollup.Day, dayStart, 10)
	if err != nil {
		t.Fatalf("TopEntities: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 entity rollup, got %d: %+v", len(results), results)
	}
	got := results[0]
	if got.EntityName != "Elon Musk" || got.EntityType != "PERSON" {
		t.Errorf("unexpected entity: %+v", got)
	}
	if got.MentionCount != 2 {
		t.Errorf("expected mention_count=2 (one per article), got %d", got.MentionCount)
	}
	if want := 0.6; got.SentimentScore < want-0.001 || got.SentimentScore > want+0.001 {
		t.Errorf("expected averaged sentiment_score=%v, got %v", want, got.SentimentScore)
	}
	if !got.WindowStart.Equal(dayStart) {
		t.Errorf("expected window_start=%v, got %v", dayStart, got.WindowStart)
	}
}

func TestRollupStore_TopEntities_RanksByMentionCountDescending(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	seedSource(t, ctx, store)

	published := time.Date(2026, time.August, 8, 9, 0, 0, 0, time.UTC)

	// Popular Corp is covered by 2 distinct articles, Niche Org by 1 — a
	// within-article repeat mention (article 1 mentions Popular Corp
	// twice) does NOT count extra, since mention_count reflects article
	// coverage, not raw occurrence density.
	articles := []pipeline.ArticleMentions{
		{
			Article: fetcher.Article{SourceID: "src-1", DedupKey: "dk-t1", URL: "https://example.com/t1", Title: "T1", Content: "body", PublishedAt: published},
			Entities: []ner.Mention{
				{Text: "Popular Corp", Type: "ORG", SentimentScore: 0.1},
				{Text: "Popular Corp", Type: "ORG", SentimentScore: 0.1},
				{Text: "Niche Org", Type: "ORG", SentimentScore: 0.1},
			},
		},
		{
			Article: fetcher.Article{SourceID: "src-1", DedupKey: "dk-t2", URL: "https://example.com/t2", Title: "T2", Content: "body", PublishedAt: published.Add(time.Hour)},
			Entities: []ner.Mention{
				{Text: "Popular Corp", Type: "ORG", SentimentScore: 0.1},
			},
		},
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}
	if err := rollupStore.Compute(ctx); err != nil {
		t.Fatalf("Compute: %v", err)
	}

	dayStart := rollup.WindowStart(rollup.Day, published)
	results, err := rollupStore.TopEntities(ctx, rollup.Day, dayStart, 1)
	if err != nil {
		t.Fatalf("TopEntities: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected limit=1 to return exactly 1 result, got %d", len(results))
	}
	if results[0].EntityName != "Popular Corp" {
		t.Errorf("expected the more-mentioned entity ranked first, got %+v", results[0])
	}
}

func TestRollupStore_ReputationTrend_ReturnsOldestFirstAcrossDays(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	seedSource(t, ctx, store)

	day1 := time.Date(2026, time.August, 6, 9, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, time.August, 7, 9, 0, 0, 0, time.UTC)

	articles := []pipeline.ArticleMentions{
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-trend-1", URL: "https://example.com/tr1", Title: "TR1", Content: "body", PublishedAt: day1},
			Entities: []ner.Mention{{Text: "Trendy Inc", Type: "ORG", SentimentScore: -0.5}},
		},
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-trend-2", URL: "https://example.com/tr2", Title: "TR2", Content: "body", PublishedAt: day2},
			Entities: []ner.Mention{{Text: "Trendy Inc", Type: "ORG", SentimentScore: 0.9}},
		},
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}
	if err := rollupStore.Compute(ctx); err != nil {
		t.Fatalf("Compute: %v", err)
	}

	var entityID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM entities WHERE name = 'Trendy Inc'`).Scan(&entityID); err != nil {
		t.Fatalf("query entity id: %v", err)
	}

	trend, err := rollupStore.ReputationTrend(ctx, entityID, rollup.Day)
	if err != nil {
		t.Fatalf("ReputationTrend: %v", err)
	}
	if len(trend) != 2 {
		t.Fatalf("expected 2 daily points, got %d", len(trend))
	}
	if trend[0].WindowStart.After(trend[1].WindowStart) {
		t.Errorf("expected oldest-first ordering, got %v then %v", trend[0].WindowStart, trend[1].WindowStart)
	}
	if trend[0].SentimentScore >= 0 || trend[1].SentimentScore <= 0 {
		t.Errorf("expected day1 negative then day2 positive, got %+v then %+v", trend[0], trend[1])
	}
}

func TestRollupStore_Compute_IsIdempotent(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	seedSource(t, ctx, store)

	published := time.Date(2026, time.August, 8, 9, 0, 0, 0, time.UTC)
	articles := []pipeline.ArticleMentions{
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-idem", URL: "https://example.com/idem", Title: "Idem", Content: "body", PublishedAt: published},
			Entities: []ner.Mention{{Text: "Repeatable Co", Type: "ORG", SentimentScore: 0.3}},
		},
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	if err := rollupStore.Compute(ctx); err != nil {
		t.Fatalf("first Compute: %v", err)
	}
	if err := rollupStore.Compute(ctx); err != nil {
		t.Fatalf("second Compute: %v", err)
	}

	dayStart := rollup.WindowStart(rollup.Day, published)
	results, err := rollupStore.TopEntities(ctx, rollup.Day, dayStart, 10)
	if err != nil {
		t.Fatalf("TopEntities: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected re-running Compute not to duplicate rows, got %d", len(results))
	}
}
