package api

import (
	"net/http"
	"strconv"
)

// handleSourceBreakdown serves the entity detail page's by-source
// breakdown (issue #24): mention count + average sentiment per source for
// one entity, across all time — which outlets cover this entity, and how
// differently they cover it.
func (h *handlers) handleSourceBreakdown(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid entity id")
		return
	}

	results, err := h.rollups.SourceBreakdown(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load source breakdown")
		return
	}

	out := make([]sourceBreakdownJSON, 0, len(results))
	for _, res := range results {
		out = append(out, toSourceBreakdownJSON(res))
	}
	writeJSON(w, http.StatusOK, out)
}
