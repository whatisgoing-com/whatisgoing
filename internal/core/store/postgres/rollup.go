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
			INSERT INTO entity_rollups (entity_id, window_kind, window_start, mention_count, sentiment_score, positive_count, neutral_count, negative_count)
			SELECT
				m.entity_id,
				$1::rollup_window,
				date_trunc($2, COALESCE(a.published_at, a.fetched_at))::date,
				SUM(m.mention_count),
				SUM(m.sentiment_score * m.mention_count) / SUM(m.mention_count),
				COUNT(*) FILTER (WHERE m.sentiment_score > 0),
				COUNT(*) FILTER (WHERE m.sentiment_score = 0),
				COUNT(*) FILTER (WHERE m.sentiment_score < 0)
			FROM mentions m
			JOIN articles a ON a.id = m.article_id
			GROUP BY m.entity_id, date_trunc($2, COALESCE(a.published_at, a.fetched_at))
			ON CONFLICT (entity_id, window_kind, window_start) DO UPDATE
			SET mention_count = EXCLUDED.mention_count,
			    sentiment_score = EXCLUDED.sentiment_score,
			    positive_count = EXCLUDED.positive_count,
			    neutral_count = EXCLUDED.neutral_count,
			    negative_count = EXCLUDED.negative_count,
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
		SELECT e.id, e.name, e.type, COALESCE(e.description, ''), r.window_kind, r.window_start, r.mention_count, r.sentiment_score, r.positive_count, r.neutral_count, r.negative_count
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

// TopEntitiesByType returns the most-mentioned entities of a single type
// (PERSON/ORG/EVENT) for a window/window start, ranked by mention_count
// descending — the home page's per-type top-10 lists (issue #32).
func (s *RollupStore) TopEntitiesByType(ctx context.Context, window rollup.Window, windowStart time.Time, entityType string, limit int) ([]rollup.EntityRollup, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.name, e.type, COALESCE(e.description, ''), r.window_kind, r.window_start, r.mention_count, r.sentiment_score, r.positive_count, r.neutral_count, r.negative_count
		FROM entity_rollups r
		JOIN entities e ON e.id = r.entity_id
		WHERE r.window_kind = $1 AND r.window_start = $2 AND e.type = $3::entity_type
		ORDER BY r.mention_count DESC
		LIMIT $4`,
		string(window), windowStart, entityType, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query top entities by type: %w", err)
	}
	defer rows.Close()

	return scanEntityRollups(rows)
}

// ReputationTrend returns an entity's mention_count/sentiment_score over
// time at a given window granularity, oldest first.
func (s *RollupStore) ReputationTrend(ctx context.Context, entityID int64, window rollup.Window) ([]rollup.EntityRollup, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.name, e.type, COALESCE(e.description, ''), r.window_kind, r.window_start, r.mention_count, r.sentiment_score, r.positive_count, r.neutral_count, r.negative_count
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
		if err := rows.Scan(&r.EntityID, &r.EntityName, &r.EntityType, &r.EntityDescription, &window, &r.WindowStart, &r.MentionCount, &r.SentimentScore, &r.PositiveCount, &r.NeutralCount, &r.NegativeCount); err != nil {
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

// SentimentBreakdown sums positive/neutral/negative mention counts across
// every entity for one window/window_start — the dashboard's overall
// sentiment pie chart. Unlike TopEntities, this isn't limited to the top
// N entities: it's a real total across everything rolled up in that
// bucket.
func (s *RollupStore) SentimentBreakdown(ctx context.Context, window rollup.Window, windowStart time.Time) (rollup.SentimentBreakdown, error) {
	var b rollup.SentimentBreakdown
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(positive_count), 0), COALESCE(SUM(neutral_count), 0), COALESCE(SUM(negative_count), 0)
		FROM entity_rollups
		WHERE window_kind = $1 AND window_start = $2`,
		string(window), windowStart,
	).Scan(&b.Positive, &b.Neutral, &b.Negative)
	if err != nil {
		return rollup.SentimentBreakdown{}, fmt.Errorf("query sentiment breakdown: %w", err)
	}
	return b, nil
}

// SourceBreakdown returns one entity's mention count + average sentiment
// per source, across all time, ranked by mention count descending — the
// entity detail page's by-source breakdown.
func (s *RollupStore) SourceBreakdown(ctx context.Context, entityID int64) ([]rollup.SourceBreakdown, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT s.id, s.name, COUNT(*), AVG(m.sentiment_score)
		FROM mentions m
		JOIN articles a ON a.id = m.article_id
		JOIN sources s ON s.id = a.source_id
		WHERE m.entity_id = $1
		GROUP BY s.id, s.name
		ORDER BY COUNT(*) DESC`,
		entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("query source breakdown: %w", err)
	}
	defer rows.Close()

	var results []rollup.SourceBreakdown
	for rows.Next() {
		var b rollup.SourceBreakdown
		if err := rows.Scan(&b.SourceID, &b.SourceName, &b.MentionCount, &b.AvgSentiment); err != nil {
			return nil, fmt.Errorf("scan source breakdown: %w", err)
		}
		results = append(results, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source breakdown: %w", err)
	}
	return results, nil
}

// RecentArticles returns the most recently published articles, newest
// first, optionally filtered to ones mentioning a single entity. entityID
// == 0 means unfiltered — article IDs are bigserial (start at 1), so 0 is
// never a real one.
func (s *RollupStore) RecentArticles(ctx context.Context, entityID int64, limit int) ([]rollup.RecentArticle, error) {
	var rows pgx.Rows
	var err error
	if entityID == 0 {
		rows, err = s.pool.Query(ctx, `
			SELECT a.id, a.title, a.url, s.name, COALESCE(a.published_at, a.fetched_at)
			FROM articles a
			JOIN sources s ON s.id = a.source_id
			ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
			LIMIT $1`,
			limit,
		)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT a.id, a.title, a.url, s.name, COALESCE(a.published_at, a.fetched_at)
			FROM articles a
			JOIN sources s ON s.id = a.source_id
			JOIN mentions m ON m.article_id = a.id AND m.entity_id = $2
			ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
			LIMIT $1`,
			limit, entityID,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("query recent articles: %w", err)
	}
	defer rows.Close()

	var results []rollup.RecentArticle
	for rows.Next() {
		var a rollup.RecentArticle
		if err := rows.Scan(&a.ID, &a.Title, &a.URL, &a.SourceName, &a.PublishedAt); err != nil {
			return nil, fmt.Errorf("scan recent article: %w", err)
		}
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent articles: %w", err)
	}
	return results, nil
}

// RelatedEntities returns the entities that co-occurred most often with
// entityID across all articles, ranked by shared-article count descending
// — the entity detail page's "related entities" section (issue #32).
// entity_cooccurrence rows are undirected (entity_a_id < entity_b_id by
// constraint), so entityID can appear on either side of a given row.
func (s *RollupStore) RelatedEntities(ctx context.Context, entityID int64, limit int) ([]rollup.RelatedEntity, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.id, e.name, e.type, COALESCE(e.description, ''), COUNT(*) AS cooccurrence_count
		FROM entity_cooccurrence c
		JOIN entities e ON e.id = CASE WHEN c.entity_a_id = $1 THEN c.entity_b_id ELSE c.entity_a_id END
		WHERE c.entity_a_id = $1 OR c.entity_b_id = $1
		GROUP BY e.id, e.name, e.type, e.description
		ORDER BY cooccurrence_count DESC
		LIMIT $2`,
		entityID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query related entities: %w", err)
	}
	defer rows.Close()

	var results []rollup.RelatedEntity
	for rows.Next() {
		var re rollup.RelatedEntity
		if err := rows.Scan(&re.ID, &re.Name, &re.Type, &re.Description, &re.CooccurrenceCount); err != nil {
			return nil, fmt.Errorf("scan related entity: %w", err)
		}
		results = append(results, re)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate related entities: %w", err)
	}
	return results, nil
}
