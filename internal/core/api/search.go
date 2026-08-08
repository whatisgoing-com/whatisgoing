package api

import "net/http"

type searchResultJSON struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	SourceID    string `json:"source_id"`
	PublishedAt string `json:"published_at"`
}

func (h *handlers) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "missing required query param: q")
		return
	}

	docs, err := h.search.Search(r.Context(), query, parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	out := make([]searchResultJSON, 0, len(docs))
	for _, doc := range docs {
		out = append(out, searchResultJSON{
			ID:          doc.ID,
			URL:         doc.URL,
			Title:       doc.Title,
			SourceID:    doc.SourceID,
			PublishedAt: doc.PublishedAt.Format("2006-01-02"),
		})
	}
	writeJSON(w, http.StatusOK, out)
}
