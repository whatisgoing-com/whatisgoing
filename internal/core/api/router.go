// Package api is core's internal JSON API, consumed by the UI/BFF service
// (issue #6) — trending entities, per-entity reputation trend, and search.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
	"github.com/whatisgoing-com/whatisgoing/internal/core/search"
)

// RollupReader is the read side of RollupStore (internal/core/store/
// postgres) that this API needs.
type RollupReader interface {
	TopEntities(ctx context.Context, window rollup.Window, windowStart time.Time, limit int) ([]rollup.EntityRollup, error)
	ReputationTrend(ctx context.Context, entityID int64, window rollup.Window) ([]rollup.EntityRollup, error)
	OverallTrend(ctx context.Context, window rollup.Window, limit int) ([]rollup.OverallTrendPoint, error)
	SearchEntities(ctx context.Context, query string, limit int) ([]rollup.EntitySummary, error)
	SentimentBreakdown(ctx context.Context, window rollup.Window, windowStart time.Time) (rollup.SentimentBreakdown, error)
}

func NewRouter(rollups RollupReader, searcher search.Searcher) http.Handler {
	h := &handlers{rollups: rollups, search: searcher}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /api/trending", h.handleTrending)
	mux.HandleFunc("GET /api/trend/overall", h.handleOverallTrend)
	mux.HandleFunc("GET /api/entities", h.handleEntitySearch)
	mux.HandleFunc("GET /api/entities/{id}", h.handleEntityTrend)
	mux.HandleFunc("GET /api/search", h.handleSearch)
	mux.HandleFunc("GET /api/sentiment", h.handleSentimentBreakdown)
	return mux
}

type handlers struct {
	rollups RollupReader
	search  search.Searcher
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
