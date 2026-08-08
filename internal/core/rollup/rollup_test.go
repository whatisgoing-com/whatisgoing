package rollup

import (
	"testing"
	"time"
)

func TestWindowStart(t *testing.T) {
	// 2026-08-08 is a Saturday; its ISO week starts Monday 2026-08-03.
	ts := time.Date(2026, time.August, 8, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		window Window
		want   time.Time
	}{
		{Day, time.Date(2026, time.August, 8, 0, 0, 0, 0, time.UTC)},
		{Week, time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)},
		{Month, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)},
		{Year, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(string(tt.window), func(t *testing.T) {
			got := WindowStart(tt.window, ts)
			if !got.Equal(tt.want) {
				t.Errorf("WindowStart(%s, %v) = %v, want %v", tt.window, ts, got, tt.want)
			}
		})
	}
}

func TestWindowStart_WeekHandlesSundayCorrectly(t *testing.T) {
	// A Sunday's ISO week still starts on the preceding Monday, not the
	// same day — this is the case Go's zero-indexed-from-Sunday Weekday()
	// makes easy to get backwards.
	sunday := time.Date(2026, time.August, 9, 0, 0, 0, 0, time.UTC)
	want := time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC)

	if got := WindowStart(Week, sunday); !got.Equal(want) {
		t.Errorf("WindowStart(Week, %v) = %v, want %v", sunday, got, want)
	}
}
