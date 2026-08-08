package api

import (
	"net/http"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
)

// handleSentimentBreakdown serves the home dashboard's overall sentiment
// pie chart: positive/neutral/negative mention counts summed across every
// entity for the current window bucket (today/this week/this month/this
// year) — a real total, not an approximation from the top-N trending
// entities.
func (h *handlers) handleSentimentBreakdown(w http.ResponseWriter, r *http.Request) {
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

	breakdown, err := h.rollups.SentimentBreakdown(r.Context(), window, windowStart)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load sentiment breakdown")
		return
	}

	writeJSON(w, http.StatusOK, sentimentBreakdownJSON{
		Positive: breakdown.Positive,
		Neutral:  breakdown.Neutral,
		Negative: breakdown.Negative,
	})
}
