package api

import (
	"net/http"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
)

// validEntityTypes matches the Postgres entity_type enum exactly.
var validEntityTypes = map[string]bool{"PERSON": true, "ORG": true, "EVENT": true}

// handleTrending serves the "hot topics/persons/orgs" feature: the
// most-mentioned entities in the current window (today/this week/this
// month/this year). An optional "type" query param (PERSON/ORG/EVENT)
// scopes the ranking to one entity type — the home page's per-type
// top-10 lists (issue #32); omitted, it ranks across all types as before.
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

	entityType := r.URL.Query().Get("type")
	var results []rollup.EntityRollup
	var err error
	if entityType == "" {
		results, err = h.rollups.TopEntities(r.Context(), window, windowStart, parseLimit(r))
	} else if validEntityTypes[entityType] {
		results, err = h.rollups.TopEntitiesByType(r.Context(), window, windowStart, entityType, parseLimit(r))
	} else {
		writeError(w, http.StatusBadRequest, "invalid type: must be one of PERSON, ORG, EVENT")
		return
	}
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
