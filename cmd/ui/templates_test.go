package main

import "testing"

func TestFormatWindowRangeLabel(t *testing.T) {
	tests := []struct {
		name        string
		window      string
		windowStart string
		windowEnd   string
		want        string
	}{
		{"day", "day", "2026-08-15", "2026-08-16", "Sat, Aug 15, 2026"},
		{"week within one month", "week", "2026-08-10", "2026-08-17", "Aug 10–16, 2026"},
		{"week spanning two months", "week", "2026-08-31", "2026-09-07", "Aug 31 – Sep 6, 2026"},
		{"month", "month", "2026-08-01", "2026-09-01", "August 2026"},
		{"year", "year", "2026-01-01", "2027-01-01", "2026"},
		{"unparseable dates", "day", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatWindowRangeLabel(tt.window, tt.windowStart, tt.windowEnd); got != tt.want {
				t.Errorf("formatWindowRangeLabel(%q, %q, %q) = %q, want %q", tt.window, tt.windowStart, tt.windowEnd, got, tt.want)
			}
		})
	}
}
