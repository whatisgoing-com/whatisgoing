package api

import (
	"net/http"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
)

// handleTrending serves the "hot topics/persons/orgs" feature: the
// most-mentioned entities in the current window (today/this week/this
// month/this year).
func (h *handlers) handleTrending(w http.ResponseWriter, r *http.Request) {
	windowParam := r.URL.Query().Get("window")
	if windowParam == "" {
		windowParam = string(rollup.Day)
	}
	window, ok := rollup.ParseWindow(windowParam)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid window: must be one of day, week, month, year")
		return
	}

	windowStart := rollup.WindowStart(window, time.Now())

	results, err := h.rollups.TopEntities(r.Context(), window, windowStart, parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load trending entities")
		return
	}

	out := make([]entityRollupJSON, 0, len(results))
	for _, res := range results {
		out = append(out, toEntityRollupJSON(res))
	}
	writeJSON(w, http.StatusOK, out)
}
