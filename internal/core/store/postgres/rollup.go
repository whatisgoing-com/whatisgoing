package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
)

// RollupStore computes and queries windowed entity-mention rollups (issue
// #5). It only needs read/write access to Postgres — unlike Store, it has
// no search-indexing responsibility, so it's a separate type rather than
// another caller of Store's indexer-carrying constructor.
type RollupStore struct {
	pool *pgxpool.Pool
}

func NewRollupStore(pool *pgxpool.Pool) *RollupStore {
	return &RollupStore{pool: pool}
}

// dateTruncUnit maps a Window to the unit date_trunc expects. Window
// values already match date_trunc's own vocabulary, but this keeps that
// coincidence from being load-bearing if either ever diverges.
var dateTruncUnit = map[rollup.Window]string{
	rollup.Day:   "day",
	rollup.Week:  "week",
	rollup.Month: "month",
	rollup.Year:  "year",
}

// Compute recomputes rollups for every window from scratch, driven by a
// scheduled k8s CronJob rather than an always-running ticker, so this
// aggregation work stays off the always-on core process. Recomputing in
// full (rather than incrementally updating only the current/in-progress
// window) keeps this correct-by-construction: at v1's ~1,000 articles/day
// volume, a full aggregation pass is cheap.
func (s *RollupStore) Compute(ctx context.Context) error {
	for _, window := range rollup.Windows {
		unit, ok := dateTruncUnit[window]
		if !ok {
			return fmt.Errorf("no date_trunc unit configured for window %q", window)
		}

		_, err := s.pool.Exec(ctx, `
			INSERT INTO entity_rollups (entity_id, window_kind, window_start, mention_count, sentiment_score)
			SELECT
				m.entity_id,
				$1::rollup_window,
				date_trunc($2, COALESCE(a.published_at, a.fetched_at))::date,
				SUM(m.mention_count),
				SUM(m.sentiment_score * m.mention_count) / SUM(m.mention_count)
			FROM mentions m
			JOIN articles a ON a.id = m.article_id
			GROUP BY m.entity_id, date_trunc($2, COALESCE(a.published_at, a.fetched_at))
			ON CONFLICT (entity_id, window_kind, window_start) DO UPDATE
			SET mention_count = EXCLUDED.mention_count,
			    sentiment_score = EXCLUDED.sentiment_score,
			    computed_at = now()`,
			string(window), unit,
		)
		if err != nil {
			return fmt.Errorf("compute %s rollups: %w", window, err)
		}
	}
	return nil
}

// TopEntities returns the most-mentioned entities for a window/window
// start, ranked by mention_count descending — the "hot topics/persons/
// orgs" feature.
func (s *RollupStore) TopEntities(ctx context.Context, window rollup.Window, windowStart time.Time, limit int) ([]rollup.EntityRollup, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.name, e.type, r.window_kind, r.window_start, r.mention_count, r.sentiment_score
		FROM entity_rollups r
		JOIN entities e ON e.id = r.entity_id
		WHERE r.window_kind = $1 AND r.window_start = $2
		ORDER BY r.mention_count DESC
		LIMIT $3`,
		string(window), windowStart, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query top entities: %w", err)
	}
	defer rows.Close()

	return scanEntityRollups(rows)
}

// ReputationTrend returns an entity's mention_count/sentiment_score over
// time at a given window granularity, oldest first.
func (s *RollupStore) ReputationTrend(ctx context.Context, entityID int64, window rollup.Window) ([]rollup.EntityRollup, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.name, e.type, r.window_kind, r.window_start, r.mention_count, r.sentiment_score
		FROM entity_rollups r
		JOIN entities e ON e.id = r.entity_id
		WHERE r.entity_id = $1 AND r.window_kind = $2
		ORDER BY r.window_start ASC`,
		entityID, string(window),
	)
	if err != nil {
		return nil, fmt.Errorf("query reputation trend: %w", err)
	}
	defer rows.Close()

	return scanEntityRollups(rows)
}

// OverallTrend returns the most recent limit window_starts' aggregate
// across every entity — total mentions and mention-count-weighted average
// sentiment — oldest first, for the home dashboard's time-series chart.
func (s *RollupStore) OverallTrend(ctx context.Context, window rollup.Window, limit int) ([]rollup.OverallTrendPoint, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT window_start, total_mentions, avg_sentiment FROM (
			SELECT
				window_start,
				SUM(mention_count) AS total_mentions,
				SUM(sentiment_score * mention_count) / SUM(mention_count) AS avg_sentiment
			FROM entity_rollups
			WHERE window_kind = $1
			GROUP BY window_start
			ORDER BY window_start DESC
			LIMIT $2
		) recent
		ORDER BY window_start ASC`,
		string(window), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query overall trend: %w", err)
	}
	defer rows.Close()

	var results []rollup.OverallTrendPoint
	for rows.Next() {
		var p rollup.OverallTrendPoint
		if err := rows.Scan(&p.WindowStart, &p.TotalMentions, &p.AvgSentiment); err != nil {
			return nil, fmt.Errorf("scan overall trend point: %w", err)
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate overall trend: %w", err)
	}
	return results, nil
}

// SearchEntities finds entities by a case-insensitive name substring
// match, for the dashboard's entity search bar.
func (s *RollupStore) SearchEntities(ctx context.Context, query string, limit int) ([]rollup.EntitySummary, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, type FROM entities
		WHERE name ILIKE '%' || $1 || '%'
		ORDER BY name
		LIMIT $2`,
		query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search entities: %w", err)
	}
	defer rows.Close()

	var results []rollup.EntitySummary
	for rows.Next() {
		var e rollup.EntitySummary
		if err := rows.Scan(&e.ID, &e.Name, &e.Type); err != nil {
			return nil, fmt.Errorf("scan entity summary: %w", err)
		}
		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity search results: %w", err)
	}
	return results, nil
}

func scanEntityRollups(rows pgx.Rows) ([]rollup.EntityRollup, error) {
	var results []rollup.EntityRollup
	for rows.Next() {
		var r rollup.EntityRollup
		var window string
		if err := rows.Scan(&r.EntityID, &r.EntityName, &r.EntityType, &window, &r.WindowStart, &r.MentionCount, &r.SentimentScore); err != nil {
			return nil, fmt.Errorf("scan entity rollup: %w", err)
		}
		r.Window = rollup.Window(window)
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity rollups: %w", err)
	}
	return results, nil
}
