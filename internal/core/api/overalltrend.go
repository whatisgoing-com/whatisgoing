package api

import (
	"net/http"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
)

type overallTrendPointJSON struct {
	WindowStart   string  `json:"window_start"`
	TotalMentions int     `json:"total_mentions"`
	AvgSentiment  float64 `json:"avg_sentiment"`
}

// handleOverallTrend serves the home dashboard's aggregate time-series
// chart: total mentions and average sentiment across every entity, per
// window_start, most recent `limit` points.
func (h *handlers) handleOverallTrend(w http.ResponseWriter, r *http.Request) {
	windowParam := r.URL.Query().Get("window")
	if windowParam == "" {
		windowParam = string(rollup.Day)
	}
	window, ok := rollup.ParseWindow(windowParam)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid window: must be one of day, week, month, year")
		return
	}

	points, err := h.rollups.OverallTrend(r.Context(), window, parseLimit(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load overall trend")
		return
	}

	out := make([]overallTrendPointJSON, 0, len(points))
	for _, p := range points {
		out = append(out, overallTrendPointJSON{
			WindowStart:   p.WindowStart.Format("2006-01-02"),
			TotalMentions: p.TotalMentions,
			AvgSentiment:  p.AvgSentiment,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
