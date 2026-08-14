// Command devseed runs one real ingestion pass against a local
// docker-compose stack, so a fresh dev environment has real data without
// waiting for cmd/core's 15-minute fetch scheduler. It's the permanent
// replacement for a disposable smoketest program that kept getting
// hand-rewritten from scratch for the same purpose.
//
// Dev-only: unlike cmd/core/ui/rollup/entity-resolver, this has no
// Dockerfile and Drone never builds or pushes it — it's meant to run on
// the host, talking to the compose stack's exposed ports.
//
// Purely additive by default: articles dedup on dedup_key, so re-running
// this never duplicates or erases anything already in the database. Pass
// -wipe (with CONFIRM=yes) for the rare case where you actually want a
// clean slate first.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/whatisgoing-com/whatisgoing/internal/core/config"
	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/ner"
	"github.com/whatisgoing-com/whatisgoing/internal/core/pipeline"
	"github.com/whatisgoing-com/whatisgoing/internal/core/politeness"
	"github.com/whatisgoing-com/whatisgoing/internal/core/search"
	postgresstore "github.com/whatisgoing-com/whatisgoing/internal/core/store/postgres"
)

const userAgent = "whatisgoingbot/0.1 (+https://whatisgoing.com; contact: hello@whatisgoing.com)"

func main() {
	wipe := flag.Bool("wipe", false, "truncate all ingested data before seeding (requires CONFIRM=yes)")
	flag.Parse()

	ctx := context.Background()

	databaseURL := envOr("DATABASE_URL", "postgres://whatisgoing:whatisgoing@localhost:5432/whatisgoing?sslmode=disable")
	meiliURL := envOr("MEILISEARCH_URL", "http://localhost:7700")
	meiliKey := envOr("MEILISEARCH_KEY", "dev-master-key")
	nerURL := envOr("NER_SENTIMENT_URL", "http://localhost:8000")
	sourcesPath := envOr("SOURCES_CONFIG_PATH", "configs/sources.yaml")

	sources, err := config.LoadSources(sourcesPath)
	if err != nil {
		log.Fatalf("load sources from %s: %v (see configs/sources.example.yaml)", sourcesPath, err)
	}
	log.Printf("loaded %d sources from %s", len(sources), sourcesPath)

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer pool.Close()

	if *wipe {
		if os.Getenv("CONFIRM") != "yes" {
			log.Fatal("-wipe requires CONFIRM=yes (e.g. CONFIRM=yes go run ./cmd/devseed -wipe) — this deletes all ingested data")
		}
		log.Println("wiping existing data (CONFIRM=yes, -wipe set)...")
		if _, err := pool.Exec(ctx, `TRUNCATE entity_rollups, entity_cooccurrence, mentions, entities, articles, sources CASCADE`); err != nil {
			log.Fatalf("wipe: %v", err)
		}
	}

	indexer := search.NewMeiliIndexer(meiliURL, meiliKey, "articles")
	store := postgresstore.NewStore(pool, indexer)

	if err := store.UpsertSources(ctx, sources); err != nil {
		log.Fatalf("upsert sources: %v", err)
	}

	politeClient := &http.Client{
		Transport: politeness.NewTransport(nil, userAgent, 1500_000_000, 2),
	}

	coordinator := &pipeline.Coordinator{
		Fetcher: &fetcher.MultiFetcher{
			RSS:  fetcher.NewRSSFetcher(politeClient),
			HTML: fetcher.NewHTMLFetcher(politeClient, 25),
		},
		NER:     ner.NewClient(nerURL, nil),
		Store:   store,
		Sources: sources,
	}

	log.Println("running one real ingestion pass (this hits real RSS feeds and the real ner-sentiment service — expect it to take a bit)...")
	if err := coordinator.Run(ctx); err != nil {
		log.Fatalf("coordinator.Run: %v", err)
	}

	var articleCount, entityCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM articles`).Scan(&articleCount); err != nil {
		log.Fatalf("count articles: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM entities`).Scan(&entityCount); err != nil {
		log.Fatalf("count entities: %v", err)
	}
	fmt.Printf("done — %d articles, %d entities in the database\n", articleCount, entityCount)
	fmt.Println("next: make run-rollup && make run-entity-resolver")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
