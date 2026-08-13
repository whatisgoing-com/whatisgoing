package postgres

import (
	"context"
	"fmt"
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

func TestRollupStore_Compute_BucketsSentimentIntoPositiveNeutralNegativeCounts(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	seedSource(t, ctx, store)

	published := time.Date(2026, time.August, 8, 9, 0, 0, 0, time.UTC)

	articles := []pipeline.ArticleMentions{
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-b1", URL: "https://example.com/b1", Title: "B1", Content: "body", PublishedAt: published},
			Entities: []ner.Mention{{Text: "Bucket Co", Type: "ORG", SentimentScore: 0.8}},
		},
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-b2", URL: "https://example.com/b2", Title: "B2", Content: "body", PublishedAt: published.Add(time.Hour)},
			Entities: []ner.Mention{{Text: "Bucket Co", Type: "ORG", SentimentScore: -0.6}},
		},
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-b3", URL: "https://example.com/b3", Title: "B3", Content: "body", PublishedAt: published.Add(2 * time.Hour)},
			Entities: []ner.Mention{{Text: "Bucket Co", Type: "ORG", SentimentScore: 0}},
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
	if got.PositiveCount != 1 || got.NeutralCount != 1 || got.NegativeCount != 1 {
		t.Errorf("expected 1 positive, 1 neutral, 1 negative, got %+v", got)
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

func TestRollupStore_OverallTrend_AggregatesAcrossEntitiesOldestFirst(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	seedSource(t, ctx, store)

	day1 := time.Date(2026, time.August, 6, 9, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, time.August, 7, 9, 0, 0, 0, time.UTC)

	articles := []pipeline.ArticleMentions{
		{
			Article: fetcher.Article{SourceID: "src-1", DedupKey: "dk-ot1", URL: "https://example.com/ot1", Title: "OT1", Content: "body", PublishedAt: day1},
			Entities: []ner.Mention{
				{Text: "Alpha", Type: "ORG", SentimentScore: 1.0},
				{Text: "Beta", Type: "ORG", SentimentScore: -1.0},
			},
		},
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-ot2", URL: "https://example.com/ot2", Title: "OT2", Content: "body", PublishedAt: day2},
			Entities: []ner.Mention{{Text: "Alpha", Type: "ORG", SentimentScore: 0.5}},
		},
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}
	if err := rollupStore.Compute(ctx); err != nil {
		t.Fatalf("Compute: %v", err)
	}

	trend, err := rollupStore.OverallTrend(ctx, rollup.Day, 10)
	if err != nil {
		t.Fatalf("OverallTrend: %v", err)
	}
	if len(trend) != 2 {
		t.Fatalf("expected 2 daily points, got %d: %+v", len(trend), trend)
	}
	if trend[0].WindowStart.After(trend[1].WindowStart) {
		t.Fatalf("expected oldest-first ordering, got %+v", trend)
	}

	// day1: Alpha(1.0) + Beta(-1.0), 2 mentions total, avg sentiment 0.
	if trend[0].TotalMentions != 2 {
		t.Errorf("expected day1 total_mentions=2, got %d", trend[0].TotalMentions)
	}
	if trend[0].AvgSentiment < -0.001 || trend[0].AvgSentiment > 0.001 {
		t.Errorf("expected day1 avg_sentiment=0, got %v", trend[0].AvgSentiment)
	}
	// day2: Alpha(0.5), 1 mention total.
	if trend[1].TotalMentions != 1 {
		t.Errorf("expected day2 total_mentions=1, got %d", trend[1].TotalMentions)
	}
	if want := 0.5; trend[1].AvgSentiment < want-0.001 || trend[1].AvgSentiment > want+0.001 {
		t.Errorf("expected day2 avg_sentiment=%v, got %v", want, trend[1].AvgSentiment)
	}
}

func TestRollupStore_OverallTrend_RespectsLimit(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	seedSource(t, ctx, store)

	base := time.Date(2026, time.August, 1, 9, 0, 0, 0, time.UTC)
	articles := make([]pipeline.ArticleMentions, 0, 5)
	for i := 0; i < 5; i++ {
		articles = append(articles, pipeline.ArticleMentions{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: fmt.Sprintf("dk-limit-%d", i), URL: fmt.Sprintf("https://example.com/limit-%d", i), Title: "L", Content: "body", PublishedAt: base.AddDate(0, 0, i)},
			Entities: []ner.Mention{{Text: "Gamma", Type: "ORG", SentimentScore: 0.1}},
		})
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}
	if err := rollupStore.Compute(ctx); err != nil {
		t.Fatalf("Compute: %v", err)
	}

	trend, err := rollupStore.OverallTrend(ctx, rollup.Day, 2)
	if err != nil {
		t.Fatalf("OverallTrend: %v", err)
	}
	if len(trend) != 2 {
		t.Fatalf("expected limit=2 to return 2 points, got %d", len(trend))
	}
	// The 2 most recent days: base+3 and base+4, still oldest-first.
	if !trend[0].WindowStart.Equal(base.AddDate(0, 0, 3).Truncate(24 * time.Hour)) {
		t.Errorf("expected the most recent 2 points, got %+v", trend)
	}
}

func TestRollupStore_SentimentBreakdown_SumsAcrossAllEntitiesNotJustTopN(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	seedSource(t, ctx, store)

	published := time.Date(2026, time.August, 8, 9, 0, 0, 0, time.UTC)

	articles := []pipeline.ArticleMentions{
		{
			Article: fetcher.Article{SourceID: "src-1", DedupKey: "dk-sb1", URL: "https://example.com/sb1", Title: "SB1", Content: "body", PublishedAt: published},
			Entities: []ner.Mention{
				{Text: "Sunny Co", Type: "ORG", SentimentScore: 0.5},
				{Text: "Stormy Co", Type: "ORG", SentimentScore: -0.5},
			},
		},
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-sb2", URL: "https://example.com/sb2", Title: "SB2", Content: "body", PublishedAt: published.Add(time.Hour)},
			Entities: []ner.Mention{{Text: "Flat Co", Type: "ORG", SentimentScore: 0}},
		},
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}
	if err := rollupStore.Compute(ctx); err != nil {
		t.Fatalf("Compute: %v", err)
	}

	dayStart := rollup.WindowStart(rollup.Day, published)

	// limit=1 would only see one entity via TopEntities, but
	// SentimentBreakdown must still see all three.
	breakdown, err := rollupStore.SentimentBreakdown(ctx, rollup.Day, dayStart)
	if err != nil {
		t.Fatalf("SentimentBreakdown: %v", err)
	}
	if breakdown.Positive != 1 || breakdown.Neutral != 1 || breakdown.Negative != 1 {
		t.Errorf("expected 1/1/1 across all entities, got %+v", breakdown)
	}
}

func TestRollupStore_SentimentBreakdown_ZeroWhenNoRollupsForWindow(t *testing.T) {
	pool := testPool(t)
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	breakdown, err := rollupStore.SentimentBreakdown(ctx, rollup.Day, time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("SentimentBreakdown: %v", err)
	}
	if breakdown != (rollup.SentimentBreakdown{}) {
		t.Errorf("expected zero breakdown for an empty window, got %+v", breakdown)
	}
}

func TestRollupStore_SearchEntities_MatchesCaseInsensitiveSubstring(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	seedSource(t, ctx, store)

	published := time.Date(2026, time.August, 8, 9, 0, 0, 0, time.UTC)
	articles := []pipeline.ArticleMentions{
		{
			Article: fetcher.Article{SourceID: "src-1", DedupKey: "dk-search-1", URL: "https://example.com/search1", Title: "S1", Content: "body", PublishedAt: published},
			Entities: []ner.Mention{
				{Text: "Elon Musk", Type: "PERSON", SentimentScore: 0.1},
				{Text: "Tesla", Type: "ORG", SentimentScore: 0.1},
			},
		},
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	results, err := rollupStore.SearchEntities(ctx, "musk", 10)
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(results) != 1 || results[0].Name != "Elon Musk" {
		t.Errorf("expected case-insensitive substring match for 'musk', got %+v", results)
	}

	noMatch, err := rollupStore.SearchEntities(ctx, "nonexistent", 10)
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(noMatch) != 0 {
		t.Errorf("expected no matches, got %+v", noMatch)
	}
}

func TestRollupStore_SourceBreakdown_PerSourceCountAndAvgSentiment(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	if err := store.UpsertSources(ctx, []fetcher.Source{
		{ID: "src-1", Name: "Source One", URL: "https://one.example.com", Type: fetcher.SourceTypeRSS},
		{ID: "src-2", Name: "Source Two", URL: "https://two.example.com", Type: fetcher.SourceTypeRSS},
	}); err != nil {
		t.Fatalf("UpsertSources: %v", err)
	}

	published := time.Date(2026, time.August, 8, 9, 0, 0, 0, time.UTC)
	articles := []pipeline.ArticleMentions{
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-sb-1", URL: "https://one.example.com/1", Title: "One A", Content: "body", PublishedAt: published},
			Entities: []ner.Mention{{Text: "Elon Musk", Type: "PERSON", SentimentScore: 0.8}},
		},
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-sb-2", URL: "https://one.example.com/2", Title: "One B", Content: "body", PublishedAt: published.Add(time.Hour)},
			Entities: []ner.Mention{{Text: "Elon Musk", Type: "PERSON", SentimentScore: 0.4}},
		},
		{
			Article:  fetcher.Article{SourceID: "src-2", DedupKey: "dk-sb-3", URL: "https://two.example.com/1", Title: "Two A", Content: "body", PublishedAt: published.Add(2 * time.Hour)},
			Entities: []ner.Mention{{Text: "Elon Musk", Type: "PERSON", SentimentScore: -0.6}},
		},
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	var entityID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM entities WHERE name = 'Elon Musk'`).Scan(&entityID); err != nil {
		t.Fatalf("look up entity id: %v", err)
	}

	results, err := rollupStore.SourceBreakdown(ctx, entityID)
	if err != nil {
		t.Fatalf("SourceBreakdown: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 sources, got %d: %+v", len(results), results)
	}

	// ordered by mention count descending: src-1 (2 mentions) before src-2 (1)
	if results[0].SourceID != "src-1" || results[0].MentionCount != 2 {
		t.Errorf("expected src-1 first with 2 mentions, got %+v", results[0])
	}
	if want := 0.6; results[0].AvgSentiment < want-0.001 || results[0].AvgSentiment > want+0.001 {
		t.Errorf("expected src-1 avg sentiment %v, got %v", want, results[0].AvgSentiment)
	}
	if results[1].SourceID != "src-2" || results[1].MentionCount != 1 {
		t.Errorf("expected src-2 second with 1 mention, got %+v", results[1])
	}
}

func TestRollupStore_RecentArticles_NewestFirstAndOptionalEntityFilter(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	rollupStore := NewRollupStore(pool)
	ctx := context.Background()

	seedSource(t, ctx, store)

	published := time.Date(2026, time.August, 8, 9, 0, 0, 0, time.UTC)
	articles := []pipeline.ArticleMentions{
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-ra-1", URL: "https://example.com/oldest", Title: "Oldest", Content: "body", PublishedAt: published},
			Entities: []ner.Mention{{Text: "Elon Musk", Type: "PERSON", SentimentScore: 0.1}},
		},
		{
			Article:  fetcher.Article{SourceID: "src-1", DedupKey: "dk-ra-2", URL: "https://example.com/newest", Title: "Newest", Content: "body", PublishedAt: published.Add(2 * time.Hour)},
			Entities: []ner.Mention{{Text: "Tesla", Type: "ORG", SentimentScore: 0.1}},
		},
	}
	if err := store.SaveArticles(ctx, articles); err != nil {
		t.Fatalf("SaveArticles: %v", err)
	}

	all, err := rollupStore.RecentArticles(ctx, 0, 10)
	if err != nil {
		t.Fatalf("RecentArticles: %v", err)
	}
	if len(all) != 2 || all[0].Title != "Newest" || all[1].Title != "Oldest" {
		t.Fatalf("expected [Newest, Oldest] unfiltered, got %+v", all)
	}
	if all[0].SourceName != "Src 1" {
		t.Errorf("expected source name Src 1, got %q", all[0].SourceName)
	}

	var teslaID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM entities WHERE name = 'Tesla'`).Scan(&teslaID); err != nil {
		t.Fatalf("look up Tesla id: %v", err)
	}

	filtered, err := rollupStore.RecentArticles(ctx, teslaID, 10)
	if err != nil {
		t.Fatalf("RecentArticles filtered: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Title != "Newest" {
		t.Fatalf("expected only [Newest] for Tesla, got %+v", filtered)
	}
}
