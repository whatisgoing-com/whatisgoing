package api

import (
	"net/http"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
)

type windowStatsJSON struct {
	ArticleCount int    `json:"article_count"`
	EntityCount  int    `json:"entity_count"`
	WindowStart  string `json:"window_start"`
	WindowEnd    string `json:"window_end"`
}

// handleWindowStats serves the home page's "Articles" and "Entities
// mentioned" stat tiles, plus the window's real start/end dates so the UI
// can show what date range the selected window actually covers.
func (h *handlers) handleWindowStats(w http.ResponseWriter, r *http.Request) {
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

	stats, err := h.rollups.WindowStats(r.Context(), window, windowStart, windowEnd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load window stats")
		return
	}

	writeJSON(w, http.StatusOK, windowStatsJSON{
		ArticleCount: stats.ArticleCount,
		EntityCount:  stats.EntityCount,
		WindowStart:  windowStart.Format("2006-01-02"),
		WindowEnd:    windowEnd.Format("2006-01-02"),
	})
}
