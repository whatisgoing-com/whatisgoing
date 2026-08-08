// Command rollup computes windowed entity-mention rollups (issue #5): hot
// topics (ranked mention frequency) and reputation trend (sentiment over
// time), for day/week/month/year windows. It runs to completion and exits
// — intended to be driven by a scheduled k8s CronJob rather than run as a
// long-lived process, keeping this aggregation work off the always-on
// core service.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	postgresstore "github.com/whatisgoing-com/whatisgoing/internal/core/store/postgres"
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

	store := postgresstore.NewRollupStore(pool)
	if err := store.Compute(ctx); err != nil {
		log.Fatalf("compute rollups: %v", err)
	}

	log.Println("rollup: computed day/week/month/year windows")
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}
