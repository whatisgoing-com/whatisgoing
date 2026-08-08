package postgres

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/search"
)

// Reconcile finds articles that were never successfully submitted for
// search indexing (indexed_at IS NULL) and retries them. It returns how
// many it repaired.
func (s *Store) Reconcile(ctx context.Context, limit int) (int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, title, content, source_id, published_at
		FROM articles
		WHERE indexed_at IS NULL
		ORDER BY id
		LIMIT $1`, limit)
	if err != nil {
		return 0, fmt.Errorf("query unindexed articles: %w", err)
	}
	defer rows.Close()

	type candidate struct {
		id  int64
		doc search.Document
	}
	var candidates []candidate

	for rows.Next() {
		var id int64
		var publishedAt *time.Time
		doc := search.Document{}

		if err := rows.Scan(&id, &doc.URL, &doc.Title, &doc.Content, &doc.SourceID, &publishedAt); err != nil {
			return 0, fmt.Errorf("scan unindexed article: %w", err)
		}
		if publishedAt != nil {
			doc.PublishedAt = *publishedAt
		}
		doc.ID = strconv.FormatInt(id, 10)

		candidates = append(candidates, candidate{id: id, doc: doc})
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate unindexed articles: %w", err)
	}

	repaired := 0
	for _, c := range candidates {
		if err := s.indexer.Index(ctx, c.doc); err != nil {
			log.Printf("reconcile: still failing to index article %d: %v", c.id, err)
			continue
		}
		if _, err := s.pool.Exec(ctx, `UPDATE articles SET indexed_at = now() WHERE id = $1`, c.id); err != nil {
			return repaired, fmt.Errorf("mark article %d indexed: %w", c.id, err)
		}
		repaired++
	}

	return repaired, nil
}
