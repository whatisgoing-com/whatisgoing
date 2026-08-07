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
	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/pipeline"
	"github.com/whatisgoing-com/whatisgoing/internal/core/scheduler"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	port := os.Getenv("CORE_PORT")
	if port == "" {
		port = "8080"
	}

	coordinator := &pipeline.Coordinator{
		Fetcher: noopFetcher{},
		Store:   logStore{},
		// Sources will be populated once source configuration lands (issue #2).
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

// noopFetcher and logStore are temporary stand-ins so the pipeline is
// wired and runnable before the real fetcher (issue #2) and Postgres-backed
// store (issue #4) exist.

type noopFetcher struct{}

func (noopFetcher) Fetch(ctx context.Context, source fetcher.Source) ([]fetcher.Article, error) {
	return nil, nil
}

type logStore struct{}

func (logStore) SaveArticles(ctx context.Context, articles []fetcher.Article) error {
	log.Printf("would save %d articles", len(articles))
	return nil
}
