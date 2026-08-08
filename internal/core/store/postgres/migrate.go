package postgres

import (
	"context"
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies any migration files under migrations/ that haven't been
// applied yet, in filename order, tracking progress in a schema_migrations
// table. Forward-only — there are no down migrations, matching the rest of
// this project's "keep v1 ops simple" approach.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if err := applyMigration(ctx, pool, name); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, name string) error {
	var alreadyApplied bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, name,
	).Scan(&alreadyApplied); err != nil {
		return fmt.Errorf("check migration %s: %w", name, err)
	}
	if alreadyApplied {
		return nil
	}

	sqlBytes, err := migrationsFS.ReadFile(path.Join("migrations", name))
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx for migration %s: %w", name, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful Commit

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}

	return nil
}
