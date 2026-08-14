package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/ner"
	"github.com/whatisgoing-com/whatisgoing/internal/core/pipeline"
	"github.com/whatisgoing-com/whatisgoing/internal/core/wikidata"
	"github.com/whatisgoing-com/whatisgoing/internal/core/wikipedia"
)

type wikidataResult struct {
	ID          string
	Label       string
	Description string
}

// fakeWikidataClient returns a wikidata.Client backed by a local server
// that answers deterministically per search term — responses map[name] ==
// zero value means "no result" (matches how a real not-found search
// looks, exercised for real against the live API while designing this).
func fakeWikidataClient(t *testing.T, responses map[string]wikidataResult) *wikidata.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("search")
		resp, ok := responses[name]
		search := []map[string]string{}
		if ok {
			search = append(search, map[string]string{"id": resp.ID, "label": resp.Label, "description": resp.Description})
		}
		json.NewEncoder(w).Encode(map[string]any{"search": search})
	}))
	t.Cleanup(server.Close)
	return wikidata.NewClient(server.URL, nil)
}

// fakeWikipediaClient returns a wikipedia.Client backed by a local server
// that 404s every request — most tests here don't care about
// descriptions, and claimCanonical treats a missing summary as fine
// (description just stays unset).
func fakeWikipediaClient(t *testing.T) *wikipedia.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)
	return wikipedia.NewClient(server.URL, nil)
}

func seedEntityWithMention(t *testing.T, ctx context.Context, store *Store, dedupKey, name, typ string, sentiment float64) {
	t.Helper()
	article := pipeline.ArticleMentions{
		Article:  fetcher.Article{SourceID: "src-1", DedupKey: dedupKey, URL: "https://example.com/" + dedupKey, Title: dedupKey, Content: "body"},
		Entities: []ner.Mention{{Text: name, Type: typ, SentimentScore: sentiment}},
	}
	if err := store.SaveArticles(ctx, []pipeline.ArticleMentions{article}); err != nil {
		t.Fatalf("seed entity %q: %v", name, err)
	}
}

func TestEntityResolver_Resolve_ClaimsFirstEntityForAQID(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	ctx := context.Background()
	seedSource(t, ctx, store)

	seedEntityWithMention(t, ctx, store, "dk-claim", "Boeing", "ORG", 0.5)

	client := fakeWikidataClient(t, map[string]wikidataResult{
		"Boeing": {ID: "Q66", Label: "Boeing", Description: "American global aerospace and defense corporation"},
	})
	resolver := NewEntityResolver(pool, client, fakeWikipediaClient(t))

	report, err := resolver.Resolve(ctx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if report.Claimed != 1 || report.Merged != 0 {
		t.Errorf("expected 1 claim, 0 merges, got %+v", report)
	}

	var wikidataID string
	if err := pool.QueryRow(ctx, `SELECT wikidata_id FROM entities WHERE name = 'Boeing' AND type = 'ORG'`).Scan(&wikidataID); err != nil {
		t.Fatalf("query wikidata_id: %v", err)
	}
	if wikidataID != "Q66" {
		t.Errorf("expected wikidata_id=Q66, got %q", wikidataID)
	}
}

func TestEntityResolver_Resolve_MergesTwoNameVariantsWithTheSameQID(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	ctx := context.Background()
	seedSource(t, ctx, store)

	// "Donald Trump" seeded first (lower id) so it's processed — and
	// claimed — before "Donald J. Trump" in the same Resolve() pass.
	seedEntityWithMention(t, ctx, store, "dk-dt1", "Donald Trump", "PERSON", 0.2)
	seedEntityWithMention(t, ctx, store, "dk-dt2", "Donald J. Trump", "PERSON", -0.6)

	client := fakeWikidataClient(t, map[string]wikidataResult{
		"Donald Trump":    {ID: "Q22686", Label: "Donald Trump", Description: "45th and 47th President of the United States"},
		"Donald J. Trump": {ID: "Q22686", Label: "Donald Trump", Description: "45th and 47th President of the United States"},
	})
	resolver := NewEntityResolver(pool, client, fakeWikipediaClient(t))

	report, err := resolver.Resolve(ctx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if report.Claimed != 1 || report.Merged != 1 {
		t.Errorf("expected 1 claim + 1 merge, got %+v", report)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM entities WHERE type = 'PERSON' AND (name = 'Donald Trump' OR name = 'Donald J. Trump')`).Scan(&count); err != nil {
		t.Fatalf("count entities: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 surviving entity, got %d", count)
	}

	var mentionCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM mentions m JOIN entities e ON e.id = m.entity_id
		WHERE e.wikidata_id = 'Q22686'`,
	).Scan(&mentionCount); err != nil {
		t.Fatalf("count mentions: %v", err)
	}
	if mentionCount != 2 {
		t.Errorf("expected both articles' mentions preserved under the merged entity, got %d", mentionCount)
	}
}

func TestEntityResolver_Resolve_FallsBackToAliasMatchWhenWikidataFindsNothing(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	ctx := context.Background()
	seedSource(t, ctx, store)

	// "Donald Trump" first (resolvable, lower id) so it's already
	// claimed by the time "Trump" (unresolvable via search — matches
	// real Wikidata behavior for a bare surname) is processed in the
	// same run.
	seedEntityWithMention(t, ctx, store, "dk-full", "Donald Trump", "PERSON", 0.2)
	seedEntityWithMention(t, ctx, store, "dk-bare", "Trump", "PERSON", -0.8)

	client := fakeWikidataClient(t, map[string]wikidataResult{
		"Donald Trump": {ID: "Q22686", Label: "Donald Trump", Description: "45th and 47th President of the United States"},
		// "Trump" intentionally absent: real API returns nothing usable.
	})
	resolver := NewEntityResolver(pool, client, fakeWikipediaClient(t))

	report, err := resolver.Resolve(ctx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if report.Claimed != 1 || report.Merged != 1 {
		t.Errorf("expected 1 claim + 1 alias-fallback merge, got %+v", report)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM entities WHERE type = 'PERSON'`).Scan(&count); err != nil {
		t.Fatalf("count entities: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 'Trump' to merge into 'Donald Trump' via the alias fallback, got %d entities", count)
	}
}

func TestEntityResolver_Resolve_LeavesAmbiguousAliasUnresolved(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	ctx := context.Background()
	seedSource(t, ctx, store)

	seedEntityWithMention(t, ctx, store, "dk-dt", "Donald Trump", "PERSON", 0.2)
	seedEntityWithMention(t, ctx, store, "dk-it", "Ivanka Trump", "PERSON", 0.3)
	seedEntityWithMention(t, ctx, store, "dk-bare", "Trump", "PERSON", -0.1)

	client := fakeWikidataClient(t, map[string]wikidataResult{
		"Donald Trump": {ID: "Q22686", Label: "Donald Trump", Description: "45th and 47th President of the United States"},
		"Ivanka Trump": {ID: "Q23685", Label: "Ivanka Trump", Description: "American businesswoman"},
		// "Trump" absent, and now ambiguous between two resolved candidates.
	})
	resolver := NewEntityResolver(pool, client, fakeWikipediaClient(t))

	if _, err := resolver.Resolve(ctx); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	var wikidataID *string
	if err := pool.QueryRow(ctx, `SELECT wikidata_id FROM entities WHERE name = 'Trump' AND type = 'PERSON'`).Scan(&wikidataID); err != nil {
		t.Fatalf("query bare Trump entity: %v", err)
	}
	if wikidataID != nil {
		t.Errorf("expected the ambiguous 'Trump' entity to stay unresolved, got wikidata_id=%q", *wikidataID)
	}
}

func TestEntityResolver_Resolve_MergesPossessiveVariantIntoExistingEntity(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	ctx := context.Background()
	seedSource(t, ctx, store)

	seedEntityWithMention(t, ctx, store, "dk-plain", "Donald Trump", "PERSON", 0.2)
	// Simulate a pre-existing possessive-noise row saved before the
	// ingestion-time fix shipped — insert it directly rather than via
	// SaveArticles, which now normalizes on the way in.
	var possessiveID int64
	if err := pool.QueryRow(ctx, `INSERT INTO entities (name, type) VALUES ('Donald Trump''s', 'PERSON') RETURNING id`).Scan(&possessiveID); err != nil {
		t.Fatalf("seed possessive entity: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO mentions (article_id, entity_id, sentiment_score, mention_count)
		SELECT id, $1, -0.5, 1 FROM articles WHERE dedup_key = 'dk-plain'`,
		possessiveID,
	); err != nil {
		t.Fatalf("seed possessive mention: %v", err)
	}

	client := fakeWikidataClient(t, map[string]wikidataResult{
		"Donald Trump": {ID: "Q22686", Label: "Donald Trump", Description: "45th and 47th President of the United States"},
	})
	resolver := NewEntityResolver(pool, client, fakeWikipediaClient(t))

	if _, err := resolver.Resolve(ctx); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM entities WHERE type = 'PERSON'`).Scan(&count); err != nil {
		t.Fatalf("count entities: %v", err)
	}
	if count != 1 {
		t.Errorf("expected the possessive variant to merge into the existing entity, got %d entities", count)
	}
}

func TestEntityResolver_Resolve_SetsDescriptionFromWikipediaWhenClaiming(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	ctx := context.Background()
	seedSource(t, ctx, store)

	seedEntityWithMention(t, ctx, store, "dk-desc", "Boeing", "ORG", 0.1)

	wdClient := fakeWikidataClient(t, map[string]wikidataResult{
		"Boeing": {ID: "Q66", Label: "Boeing", Description: "American global aerospace and defense corporation"},
	})
	wpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Boeing" {
			t.Fatalf("unexpected wikipedia path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{"extract": "The Boeing Company is an American multinational corporation."})
	}))
	defer wpServer.Close()

	resolver := NewEntityResolver(pool, wdClient, wikipedia.NewClient(wpServer.URL, nil))
	if _, err := resolver.Resolve(ctx); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	var description *string
	if err := pool.QueryRow(ctx, `SELECT description FROM entities WHERE name = 'Boeing' AND type = 'ORG'`).Scan(&description); err != nil {
		t.Fatalf("query description: %v", err)
	}
	if description == nil || *description != "The Boeing Company is an American multinational corporation." {
		t.Errorf("expected the Wikipedia extract to be stored, got %v", description)
	}
}

func TestEntityResolver_Resolve_LeavesDescriptionUnsetWhenWikipediaHasNoPage(t *testing.T) {
	pool := testPool(t)
	store := NewStore(pool, &fakeIndexer{})
	ctx := context.Background()
	seedSource(t, ctx, store)

	seedEntityWithMention(t, ctx, store, "dk-nodesc", "Boeing", "ORG", 0.1)

	wdClient := fakeWikidataClient(t, map[string]wikidataResult{
		"Boeing": {ID: "Q66", Label: "Boeing", Description: "American global aerospace and defense corporation"},
	})
	resolver := NewEntityResolver(pool, wdClient, fakeWikipediaClient(t)) // 404s everything

	if _, err := resolver.Resolve(ctx); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	var wikidataID string
	var description *string
	if err := pool.QueryRow(ctx, `SELECT wikidata_id, description FROM entities WHERE name = 'Boeing' AND type = 'ORG'`).Scan(&wikidataID, &description); err != nil {
		t.Fatalf("query entity: %v", err)
	}
	if wikidataID != "Q66" {
		t.Errorf("expected the entity to still be resolved despite no Wikipedia page, got wikidata_id=%q", wikidataID)
	}
	if description != nil {
		t.Errorf("expected no description, got %q", *description)
	}
}
