package api

import "net/http"

type entitySummaryJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// handleEntitySearch serves the dashboard's entity search bar: find
// entities by a case-insensitive name substring match.
func (h *handlers) handleEntitySearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing required query param: q")
		return
	}

	results, err := h.rollups.SearchEntities(r.Context(), query, parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "entity search failed")
		return
	}

	out := make([]entitySummaryJSON, 0, len(results))
	for _, e := range results {
		out = append(out, entitySummaryJSON{ID: e.ID, Name: e.Name, Type: e.Type})
	}
	writeJSON(w, http.StatusOK, out)
}
