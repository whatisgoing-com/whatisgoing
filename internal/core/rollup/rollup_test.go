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

func TestWindowEnd(t *testing.T) {
	tests := []struct {
		window Window
		start  time.Time
		want   time.Time
	}{
		{Day, time.Date(2026, time.August, 8, 0, 0, 0, 0, time.UTC), time.Date(2026, time.August, 9, 0, 0, 0, 0, time.UTC)},
		{Week, time.Date(2026, time.August, 3, 0, 0, 0, 0, time.UTC), time.Date(2026, time.August, 10, 0, 0, 0, 0, time.UTC)},
		{Month, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)},
		{Year, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC), time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(string(tt.window), func(t *testing.T) {
			got := WindowEnd(tt.window, tt.start)
			if !got.Equal(tt.want) {
				t.Errorf("WindowEnd(%s, %v) = %v, want %v", tt.window, tt.start, got, tt.want)
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

func TestParseWindow(t *testing.T) {
	for _, s := range []string{"day", "week", "month", "year"} {
		t.Run(s, func(t *testing.T) {
			w, ok := ParseWindow(s)
			if !ok || string(w) != s {
				t.Errorf("ParseWindow(%q) = (%q, %v), want (%q, true)", s, w, ok, s)
			}
		})
	}

	if _, ok := ParseWindow("decade"); ok {
		t.Error("ParseWindow(\"decade\") should reject an unknown window")
	}
}
