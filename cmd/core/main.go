package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/api"
	"github.com/whatisgoing-com/whatisgoing/internal/core/config"
	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/pipeline"
	"github.com/whatisgoing-com/whatisgoing/internal/core/politeness"
	"github.com/whatisgoing-com/whatisgoing/internal/core/scheduler"
)

const (
	userAgent          = "whatisgoingbot/0.1 (+https://whatisgoing.com; contact: hello@whatisgoing.com)"
	minDelayPerDomain  = 1500 * time.Millisecond
	maxConcurrentFetch = 2
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	port := os.Getenv("CORE_PORT")
	if port == "" {
		port = "8080"
	}

	sourcesPath := os.Getenv("SOURCES_CONFIG_PATH")
	if sourcesPath == "" {
		sourcesPath = "configs/sources.yaml"
	}
	sources, err := config.LoadSources(sourcesPath)
	if err != nil {
		log.Printf("no sources loaded from %s (%v) — ingestion will be a no-op until one is provided", sourcesPath, err)
	}

	politeClient := &http.Client{
		Transport: politeness.NewTransport(nil, userAgent, minDelayPerDomain, maxConcurrentFetch),
	}

	coordinator := &pipeline.Coordinator{
		Fetcher: &fetcher.MultiFetcher{
			RSS:  fetcher.NewRSSFetcher(politeClient),
			HTML: fetcher.NewHTMLFetcher(politeClient, 25),
		},
		Store:   logStore{},
		Sources: sources,
	}

	sched := &scheduler.Scheduler{
		Interval: 15 * time.Minute,
		Task:     coordinator.Run,
	}
	go sched.Run(ctx)

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

// logStore is a temporary stand-in for the Postgres-backed store (issue #4).
type logStore struct{}

func (logStore) SaveArticles(ctx context.Context, articles []fetcher.Article) error {
	log.Printf("would save %d articles", len(articles))
	return nil
}
