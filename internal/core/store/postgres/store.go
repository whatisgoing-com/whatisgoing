package postgres

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
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
			return err
		}
	}
	return nil
}

func (s *Store) SaveArticles(ctx context.Context, articles []fetcher.Article) error {
	for _, article := range articles {
		id, inserted, err := s.insertArticle(ctx, article)
		if err != nil {
			return err
		}
		if !inserted {
			// Already existed from a previous run; nothing new to index.
			continue
		}

		doc := search.Document{
			ID:          strconv.FormatInt(id, 10),
			URL:         article.URL,
			Title:       article.Title,
			Content:     article.Content,
			SourceID:    article.SourceID,
			PublishedAt: article.PublishedAt,
		}
		if err := s.indexer.Index(ctx, doc); err != nil {
			log.Printf("store: failed to index article %d (will be retried by reconciliation): %v", id, err)
			continue
		}
		if _, err := s.pool.Exec(ctx, `UPDATE articles SET indexed_at = now() WHERE id = $1`, id); err != nil {
			log.Printf("store: indexed article %d but failed to record indexed_at: %v", id, err)
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
