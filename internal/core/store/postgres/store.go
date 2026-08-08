package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
	"github.com/whatisgoing-com/whatisgoing/internal/core/ner"
	"github.com/whatisgoing-com/whatisgoing/internal/core/pipeline"
	"github.com/whatisgoing-com/whatisgoing/internal/core/search"
)

// Store persists articles to Postgres and dual-writes them to the search
// index. Postgres is authoritative: a search-indexing failure is logged and
// left for Reconcile to retry, not treated as a failure of SaveArticles.
type Store struct {
	pool    *pgxpool.Pool
	indexer search.Indexer
}

func NewStore(pool *pgxpool.Pool, indexer search.Indexer) *Store {
	return &Store{pool: pool, indexer: indexer}
}

// UpsertSources ensures every configured source has a matching row, so
// articles can satisfy their foreign key. Called once at startup.
func (s *Store) UpsertSources(ctx context.Context, sources []fetcher.Source) error {
	for _, src := range sources {
		_, err := s.pool.Exec(ctx, `
			INSERT INTO sources (id, name, url, type)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE SET name = $2, url = $3, type = $4`,
			src.ID, src.Name, src.URL, string(src.Type),
		)
		if err != nil {
			return fmt.Errorf("upsert source %s: %w", src.ID, err)
		}
	}
	return nil
}

func (s *Store) SaveArticles(ctx context.Context, articles []pipeline.ArticleMentions) error {
	for _, am := range articles {
		id, inserted, err := s.insertArticle(ctx, am.Article)
		if err != nil {
			return err
		}
		if !inserted {
			// Already existed from a previous run; nothing new to index or
			// extract entities for.
			continue
		}

		doc := search.Document{
			ID:          strconv.FormatInt(id, 10),
			URL:         am.Article.URL,
			Title:       am.Article.Title,
			Content:     am.Article.Content,
			SourceID:    am.Article.SourceID,
			PublishedAt: am.Article.PublishedAt,
		}
		if err := s.indexer.Index(ctx, doc); err != nil {
			log.Printf("store: failed to index article %d (will be retried by reconciliation): %v", id, err)
		} else if _, err := s.pool.Exec(ctx, `UPDATE articles SET indexed_at = now() WHERE id = $1`, id); err != nil {
			log.Printf("store: indexed article %d but failed to record indexed_at: %v", id, err)
		}

		if err := s.saveMentions(ctx, id, am.Entities); err != nil {
			log.Printf("store: failed to save entity mentions for article %d: %v", id, err)
		}
	}

	return nil
}

// insertArticle returns the article's id and whether this call actually
// inserted a new row (false if dedup_key already existed).
func (s *Store) insertArticle(ctx context.Context, article fetcher.Article) (int64, bool, error) {
	var publishedAt *time.Time
	if !article.PublishedAt.IsZero() {
		publishedAt = &article.PublishedAt
	}

	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO articles (source_id, dedup_key, url, title, content, published_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (dedup_key) DO NOTHING
		RETURNING id`,
		article.SourceID, article.DedupKey, article.URL, article.Title, article.Content, publishedAt,
	).Scan(&id)

	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	return id, true, nil
}

type entityKey struct {
	name string
	typ  string
}

// saveMentions upserts every distinct entity mentioned in the article and
// writes one mentions row per (article, entity) — mention_count is always
// 1 regardless of how many times the entity was mentioned within this one
// article (that only affects sentiment_score, averaged across those
// occurrences): a mention is "this entity appeared in this article", not
// a count of in-article repetitions, so that rollups summing mention_count
// across articles measure how many articles covered the entity rather than
// letting one repetitive article dominate the ranking. It also records
// co-occurrence between every pair of distinct entities mentioned in the
// article.
func (s *Store) saveMentions(ctx context.Context, articleID int64, mentions []ner.Mention) error {
	if len(mentions) == 0 {
		return nil
	}

	type aggregate struct {
		occurrences  int
		sentimentSum float64
	}

	aggregates := make(map[entityKey]*aggregate)
	order := make([]entityKey, 0, len(mentions))

	for _, m := range mentions {
		if m.Text == "" || m.Type == "" {
			continue
		}
		key := entityKey{name: m.Text, typ: m.Type}
		a, ok := aggregates[key]
		if !ok {
			a = &aggregate{}
			aggregates[key] = a
			order = append(order, key)
		}
		a.occurrences++
		a.sentimentSum += m.SentimentScore
	}

	entityIDs := make([]int64, 0, len(order))
	for _, key := range order {
		a := aggregates[key]

		entityID, err := s.upsertEntity(ctx, key.name, key.typ)
		if err != nil {
			return fmt.Errorf("upsert entity %s (%s): %w", key.name, key.typ, err)
		}

		sentiment := a.sentimentSum / float64(a.occurrences)
		const mentionCount = 1
		if _, err := s.pool.Exec(ctx, `
			INSERT INTO mentions (article_id, entity_id, sentiment_score, mention_count)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (article_id, entity_id) DO UPDATE SET sentiment_score = $3, mention_count = $4`,
			articleID, entityID, sentiment, mentionCount,
		); err != nil {
			return fmt.Errorf("save mention for entity %d: %w", entityID, err)
		}

		entityIDs = append(entityIDs, entityID)
	}

	return s.saveCooccurrences(ctx, articleID, entityIDs)
}

func (s *Store) upsertEntity(ctx context.Context, name, entityType string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO entities (name, type)
		VALUES ($1, $2::entity_type)
		ON CONFLICT (name, type) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`,
		name, entityType,
	).Scan(&id)
	return id, err
}

// saveCooccurrences records one row per canonical-ordered pair of distinct
// entities mentioned in the same article.
func (s *Store) saveCooccurrences(ctx context.Context, articleID int64, entityIDs []int64) error {
	for i := 0; i < len(entityIDs); i++ {
		for j := i + 1; j < len(entityIDs); j++ {
			a, b := entityIDs[i], entityIDs[j]
			if a == b {
				continue
			}
			if a > b {
				a, b = b, a
			}
			if _, err := s.pool.Exec(ctx, `
				INSERT INTO entity_cooccurrence (article_id, entity_a_id, entity_b_id)
				VALUES ($1, $2, $3)
				ON CONFLICT (article_id, entity_a_id, entity_b_id) DO NOTHING`,
				articleID, a, b,
			); err != nil {
				return fmt.Errorf("save cooccurrence (%d,%d): %w", a, b, err)
			}
		}
	}
	return nil
}
