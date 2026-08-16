package main

import "testing"

func TestTrendGranularity(t *testing.T) {
	tests := []struct {
		window    string
		wantGrain string
		wantLimit int
	}{
		{"year", "month", 12},
		{"month", "week", 5},
		{"week", "day", 7},
		{"day", "day", 7},
		{"", "day", 7},
	}
	for _, tt := range tests {
		t.Run(tt.window, func(t *testing.T) {
			gotGrain, gotLimit := trendGranularity(tt.window)
			if gotGrain != tt.wantGrain || gotLimit != tt.wantLimit {
				t.Errorf("trendGranularity(%q) = (%q, %d), want (%q, %d)", tt.window, gotGrain, gotLimit, tt.wantGrain, tt.wantLimit)
			}
		})
	}
}
