package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/whatisgoing-com/whatisgoing/internal/core/api"
	"github.com/whatisgoing-com/whatisgoing/internal/core/config"
	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/ner"
	"github.com/whatisgoing-com/whatisgoing/internal/core/pipeline"
	"github.com/whatisgoing-com/whatisgoing/internal/core/politeness"
	"github.com/whatisgoing-com/whatisgoing/internal/core/scheduler"
	"github.com/whatisgoing-com/whatisgoing/internal/core/search"
	postgresstore "github.com/whatisgoing-com/whatisgoing/internal/core/store/postgres"
)

const (
	userAgent          = "whatisgoingbot/0.1 (+https://whatisgoing.com; contact: hello@whatisgoing.com)"
	minDelayPerDomain  = 1500 * time.Millisecond
	maxConcurrentFetch = 2
	fetchInterval      = 15 * time.Minute
	reconcileInterval  = time.Hour
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	port := envOr("CORE_PORT", "8080")

	sourcesPath := envOr("SOURCES_CONFIG_PATH", "configs/sources.yaml")
	sources, err := config.LoadSources(sourcesPath)
	if err != nil {
		log.Printf("no sources loaded from %s (%v) — ingestion will be a no-op until one is provided", sourcesPath, err)
	}

	databaseURL := mustEnv("DATABASE_URL")

	if err := postgresstore.Migrate(ctx, databaseURL); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer pool.Close()

	indexer := search.NewMeiliIndexer(mustEnv("MEILISEARCH_URL"), os.Getenv("MEILISEARCH_KEY"), "articles")
	store := postgresstore.NewStore(pool, indexer)

	if err := store.UpsertSources(ctx, sources); err != nil {
		log.Fatalf("upsert sources: %v", err)
	}

	politeClient := &http.Client{
		Transport: politeness.NewTransport(nil, userAgent, minDelayPerDomain, maxConcurrentFetch),
	}

	nerClient := ner.NewClient(mustEnv("NER_SENTIMENT_URL"), nil)

	coordinator := &pipeline.Coordinator{
		Fetcher: &fetcher.MultiFetcher{
			RSS:  fetcher.NewRSSFetcher(politeClient),
			HTML: fetcher.NewHTMLFetcher(politeClient, 25),
		},
		NER:     nerClient,
		Store:   store,
		Sources: sources,
	}

	fetchSched := &scheduler.Scheduler{Interval: fetchInterval, Task: coordinator.Run}
	go fetchSched.Run(ctx)

	reconcileSched := &scheduler.Scheduler{
		Interval: reconcileInterval,
		Task: func(ctx context.Context) error {
			repaired, err := store.Reconcile(ctx, 100)
			if repaired > 0 {
				log.Printf("reconcile: repaired %d articles missing from the search index", repaired)
			}
			return err
		},
	}
	go reconcileSched.Run(ctx)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: api.NewRouter(),
	}

	go func() {
		log.Printf("core service listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}
