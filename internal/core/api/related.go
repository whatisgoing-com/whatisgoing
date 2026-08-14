package api

import (
	"net/http"
	"strconv"
)

// handleRelatedEntities serves the entity detail page's "related
// entities" section (issue #32): the entities that co-occurred most
// often with this one across articles, ranked by shared-article count.
func (h *handlers) handleRelatedEntities(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid entity id")
		return
	}

	results, err := h.rollups.RelatedEntities(r.Context(), id, parseLimit(r))
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
