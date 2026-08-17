package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
)

// handleRelatedEntities serves the entity detail page's "related
// entities" section (issue #32): the entities that co-occurred most
// often with this one within the selected window, ranked by
// shared-article count (windowed + grouped-by-type client-side, issue
// #62).
func (h *handlers) handleRelatedEntities(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid entity id")
		return
	}

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
	windowEnd := rollup.WindowEnd(window, windowStart)

	results, err := h.rollups.RelatedEntities(r.Context(), id, windowStart, windowEnd, parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load related entities")
		return
	}

	out := make([]relatedEntityJSON, 0, len(results))
	for _, res := range results {
		out = append(out, toRelatedEntityJSON(res))
	}
	writeJSON(w, http.StatusOK, out)
}
