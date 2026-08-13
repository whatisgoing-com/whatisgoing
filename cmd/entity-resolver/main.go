// Command entity-resolver canonicalizes entity name variants (issue #26)
// — "Trump", "Donald Trump", and "Donald J. Trump" collapsing into one
// entity instead of three competing separately for "top entities"
// ranking — by resolving each not-yet-resolved entity against Wikidata
// and merging ones that turn out to be the same real-world item. It runs
// to completion and exits, same shape as cmd/rollup: intended to be
// driven by a schedule (a k8s CronJob, eventually) rather than run as a
// long-lived process.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresstore "github.com/whatisgoing-com/whatisgoing/internal/core/store/postgres"
	"github.com/whatisgoing-com/whatisgoing/internal/core/wikidata"
	"github.com/whatisgoing-com/whatisgoing/internal/core/wikipedia"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseURL := mustEnv("DATABASE_URL")

	if err := postgresstore.Migrate(ctx, databaseURL); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer pool.Close()

	resolver := postgresstore.NewEntityResolver(
		pool,
		wikidata.NewClient(wikidata.DefaultBaseURL, nil),
		wikipedia.NewClient(wikipedia.DefaultBaseURL, nil),
	)
	report, err := resolver.Resolve(ctx)
	if err != nil {
		log.Fatalf("resolve entities: %v", err)
	}

	log.Printf("entity-resolver: processed %d, claimed %d, merged %d", report.Processed, report.Claimed, report.Merged)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}
