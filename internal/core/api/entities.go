package api

import (
	"net/http"
	"strconv"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
)

type entityDetailJSON struct {
	ID    int64              `json:"id"`
	Name  string             `json:"name"`
	Type  string             `json:"type"`
	Trend []entityRollupJSON `json:"trend"`
}

// handleEntityTrend serves an entity's detail + "reputation trend":
// mention count and sentiment over time at a given window granularity.
// Entity name/type are derived from the trend results themselves (the
// rollup query already joins entities), so an entity with no rollups yet
// — e.g. seen by the pipeline but not yet processed by the batch
// aggregation job — 404s rather than returning a name-less stub.
func (h *handlers) handleEntityTrend(w http.ResponseWriter, r *http.Request) {
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

	trend, err := h.rollups.ReputationTrend(r.Context(), id, window)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load reputation trend")
		return
	}
	if len(trend) == 0 {
		writeError(w, http.StatusNotFound, "entity not found or has no rollups yet")
		return
	}

	out := entityDetailJSON{
		ID:    trend[0].EntityID,
		Name:  trend[0].EntityName,
		Type:  trend[0].EntityType,
		Trend: make([]entityRollupJSON, 0, len(trend)),
	}
	for _, point := range trend {
		out.Trend = append(out.Trend, toEntityRollupJSON(point))
	}
	writeJSON(w, http.StatusOK, out)
}
