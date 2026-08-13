package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/whatisgoing-com/whatisgoing/internal/core/entityname"
	"github.com/whatisgoing-com/whatisgoing/internal/core/wikidata"
	"github.com/whatisgoing-com/whatisgoing/internal/core/wikipedia"
)

// EntityResolver canonicalizes entity name variants (issue #26): "Trump",
// "Donald Trump", and "Donald J. Trump" should all roll up into one
// entity rather than competing separately for "top entities" ranking.
// Two entities that resolve to the same Wikidata item (by QID) are
// merged; a name Wikidata can't confidently resolve (typically a bare
// surname — see wikidata.Client.Search) falls back to matching against
// already-resolved entities of the same type by name-token containment
// (e.g. "Trump" ⊆ "Donald Trump"), only when that match is unambiguous.
// Whichever entity becomes canonical for a QID also gets a short
// descriptive paragraph from Wikipedia, for the entity detail page.
type EntityResolver struct {
	pool      *pgxpool.Pool
	wikidata  *wikidata.Client
	wikipedia *wikipedia.Client
}

func NewEntityResolver(pool *pgxpool.Pool, wikidataClient *wikidata.Client, wikipediaClient *wikipedia.Client) *EntityResolver {
	return &EntityResolver{pool: pool, wikidata: wikidataClient, wikipedia: wikipediaClient}
}

const (
	resolveBatchLimit  = 200
	wikidataRequestGap = 200 * time.Millisecond
)

// ResolveReport summarizes one Resolve run.
type ResolveReport struct {
	Processed int
	Claimed   int // got a wikidata_id for the first time (no existing entity to merge into)
	Merged    int // merged into an existing entity and was deleted
}

// Resolve processes up to resolveBatchLimit not-yet-resolved entities.
// Safe to re-run repeatedly (a k8s CronJob, eventually): entities already
// carrying a wikidata_id are never re-queried. A single entity's lookup
// failing (e.g. Wikidata briefly unreachable) is logged and skipped
// rather than aborting the whole run, matching Store.Reconcile's
// established pattern for this kind of best-effort background job.
func (r *EntityResolver) Resolve(ctx context.Context) (ResolveReport, error) {
	var report ResolveReport

	rows, err := r.pool.Query(ctx, `
		SELECT id, name, type FROM entities
		WHERE wikidata_id IS NULL
		ORDER BY id
		LIMIT $1`,
		resolveBatchLimit,
	)
	if err != nil {
		return report, fmt.Errorf("query unresolved entities: %w", err)
	}
	type candidate struct {
		id   int64
		name string
		typ  string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.name, &c.typ); err != nil {
			rows.Close()
			return report, fmt.Errorf("scan candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return report, fmt.Errorf("iterate candidates: %w", err)
	}

	for i, c := range candidates {
		if i > 0 {
			select {
			case <-ctx.Done():
				return report, ctx.Err()
			case <-time.After(wikidataRequestGap):
			}
		}
		if err := r.resolveOne(ctx, c.id, c.name, c.typ, &report); err != nil {
			return report, fmt.Errorf("resolve entity %d (%q): %w", c.id, c.name, err)
		}
		report.Processed++
	}

	return report, nil
}

func (r *EntityResolver) resolveOne(ctx context.Context, id int64, rawName, typ string, report *ResolveReport) error {
	name := entityname.Normalize(rawName)

	// A normalized name might already match another (possibly also
	// unresolved) entity row directly — collapse into it before doing
	// any Wikidata work, so e.g. a stale "Donald Trump's" row (saved
	// before ingestion-time normalization shipped) merges into "Donald
	// Trump" even if neither has been resolved yet.
	if name != rawName {
		merged, err := r.mergeIfNameExists(ctx, id, name, typ, report)
		if err != nil {
			return err
		}
		if merged {
			return nil
		}
		if _, err := r.pool.Exec(ctx, `UPDATE entities SET name = $1 WHERE id = $2`, name, id); err != nil {
			return fmt.Errorf("rename entity %d to normalized form: %w", id, err)
		}
	}

	match, found, err := r.wikidata.Search(ctx, name)
	if err != nil {
		log.Printf("entity-resolver: wikidata search failed for entity %d (%q): %v", id, name, err)
		return nil
	}
	if !found {
		return r.mergeByAlias(ctx, id, name, typ, report)
	}

	// An existing entity already carrying this exact QID is authoritative.
	existingID, ok, err := r.findEntity(ctx, "wikidata_id = $1 AND type = $2 AND id != $3", match.ID, typ, id)
	if err != nil {
		return err
	}
	if ok {
		return r.finishMerge(ctx, existingID, id, report)
	}

	// Otherwise, another not-yet-resolved row might already hold the
	// canonical label — attach the QID to that row and merge into it,
	// rather than creating two rows for the same QID.
	existingID, ok, err = r.findEntity(ctx, "name = $1 AND type = $2 AND id != $3", match.Label, typ, id)
	if err != nil {
		return err
	}
	if ok {
		if err := r.claimCanonical(ctx, existingID, match); err != nil {
			return err
		}
		return r.finishMerge(ctx, existingID, id, report)
	}

	// No existing entity for this QID: this row becomes canonical.
	if err := r.claimCanonical(ctx, id, match); err != nil {
		return err
	}
	report.Claimed++
	return nil
}

// claimCanonical marks entityID as the canonical row for match: sets its
// wikidata_id and renames it to Wikidata's label, then fetches a short
// descriptive paragraph from Wikipedia for the entity detail page. A
// failed or missing Wikipedia lookup doesn't block claiming the QID —
// the entity is still correctly resolved/deduplicated either way, just
// without a description for now (there's no separate backfill pass, but
// nothing prevents a future run from retrying: description only ever
// gets set here, and this is a fresh column with no legacy unresolved
// rows to backfill as of this PR).
func (r *EntityResolver) claimCanonical(ctx context.Context, entityID int64, match wikidata.Match) error {
	description, found, err := r.wikipedia.Summary(ctx, match.Label)
	if err != nil {
		log.Printf("entity-resolver: wikipedia summary failed for %q: %v", match.Label, err)
	}
	if !found {
		description = ""
	}

	if _, err := r.pool.Exec(ctx, `
		UPDATE entities SET wikidata_id = $1, name = $2, description = NULLIF($3, '')
		WHERE id = $4`,
		match.ID, match.Label, description, entityID,
	); err != nil {
		return fmt.Errorf("claim wikidata_id for entity %d: %w", entityID, err)
	}
	return nil
}

func (r *EntityResolver) mergeIfNameExists(ctx context.Context, id int64, name, typ string, report *ResolveReport) (bool, error) {
	existingID, ok, err := r.findEntity(ctx, "name = $1 AND type = $2 AND id != $3", name, typ, id)
	if err != nil || !ok {
		return false, err
	}
	if err := r.finishMerge(ctx, existingID, id, report); err != nil {
		return false, err
	}
	return true, nil
}

// mergeByAlias handles names Wikidata couldn't confidently resolve
// (typically a bare surname or first name) by checking whether exactly
// one already-resolved entity of the same type has this name as a subset
// of its own name's tokens (e.g. "Trump" ⊆ "Donald Trump"). Multiple
// matches (genuinely ambiguous — e.g. two different resolved people who
// share a surname) or no matches leave the entity unresolved rather than
// guessing wrong.
func (r *EntityResolver) mergeByAlias(ctx context.Context, id int64, name, typ string, report *ResolveReport) error {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM entities WHERE type = $1 AND wikidata_id IS NOT NULL AND id != $2`, typ, id)
	if err != nil {
		return fmt.Errorf("query resolved entities for alias match: %w", err)
	}
	defer rows.Close()

	needle := tokenize(name)
	var matchID int64
	matches := 0
	for rows.Next() {
		var rid int64
		var rname string
		if err := rows.Scan(&rid, &rname); err != nil {
			return fmt.Errorf("scan resolved entity: %w", err)
		}
		if containsAllTokens(tokenize(rname), needle) {
			matchID = rid
			matches++
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate resolved entities: %w", err)
	}

	if matches != 1 {
		return nil
	}
	return r.finishMerge(ctx, matchID, id, report)
}

func (r *EntityResolver) findEntity(ctx context.Context, where string, args ...any) (int64, bool, error) {
	var id int64
	err := r.pool.QueryRow(ctx, "SELECT id FROM entities WHERE "+where, args...).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("look up entity: %w", err)
	}
	return id, true, nil
}

func (r *EntityResolver) finishMerge(ctx context.Context, keepID, dropID int64, report *ResolveReport) error {
	if err := r.mergeInto(ctx, keepID, dropID); err != nil {
		return err
	}
	report.Merged++
	return nil
}

// mergeInto merges dropID into keepID: migrates its mentions (averaging
// sentiment for any article both already have a mention row for), then
// deletes dropID. entity_cooccurrence and entity_rollups rows for dropID
// cascade-delete automatically via their foreign keys — nothing reads
// entity_cooccurrence yet, so losing dropID's raw co-occurrence facts
// (rather than remapping them onto keepID) is an accepted simplification;
// entity_rollups gets fully recomputed by the next rollup run regardless.
func (r *EntityResolver) mergeInto(ctx context.Context, keepID, dropID int64) error {
	if keepID == dropID {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin merge transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO mentions (article_id, entity_id, sentiment_score, mention_count)
		SELECT article_id, $1, sentiment_score, mention_count FROM mentions WHERE entity_id = $2
		ON CONFLICT (article_id, entity_id) DO UPDATE
		SET sentiment_score = (mentions.sentiment_score + EXCLUDED.sentiment_score) / 2`,
		keepID, dropID,
	); err != nil {
		return fmt.Errorf("migrate mentions from %d to %d: %w", dropID, keepID, err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM entities WHERE id = $1`, dropID); err != nil {
		return fmt.Errorf("delete merged entity %d: %w", dropID, err)
	}

	return tx.Commit(ctx)
}

func tokenize(name string) map[string]bool {
	tokens := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(name)) {
		tokens[w] = true
	}
	return tokens
}

func containsAllTokens(haystack, needle map[string]bool) bool {
	for t := range needle {
		if !haystack[t] {
			return false
		}
	}
	return true
}
