package api

import (
	"net/http"
	"strconv"
)

// handleRecentArticles serves the recent-articles list (issue #24):
// headline, source, link, publish time — newest first, optionally
// filtered to articles mentioning one entity via ?entity_id=.
func (h *handlers) handleRecentArticles(w http.ResponseWriter, r *http.Request) {
	var entityID int64
	if raw := r.URL.Query().Get("entity_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid entity_id")
			return
		}
		entityID = id
	}

	results, err := h.rollups.RecentArticles(r.Context(), entityID, parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load recent articles")
		return
	}

	out := make([]recentArticleJSON, 0, len(results))
	for _, res := range results {
		out = append(out, toRecentArticleJSON(res))
	}
	writeJSON(w, http.StatusOK, out)
}
