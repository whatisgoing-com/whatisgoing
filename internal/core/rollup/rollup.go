// Package rollup defines the windowed entity-mention aggregates (issue
// #5) that power "hot topics" (ranked mention frequency) and per-entity
// "reputation trend" (sentiment over time). Computation and querying live
// in internal/core/store/postgres — this package only holds the shared
// vocabulary between them.
package rollup

import "time"

// Window is a rollup granularity. Values match the Postgres rollup_window
// enum exactly.
type Window string

const (
	Day   Window = "day"
	Week  Window = "week"
	Month Window = "month"
	Year  Window = "year"
)

// Windows lists every granularity a full recompute covers.
var Windows = []Window{Day, Week, Month, Year}

// EntityRollup is one (entity, window, window start) aggregate: how many
// times the entity was mentioned in that window, and its average
// sentiment.
type EntityRollup struct {
	EntityID       int64
	EntityName     string
	EntityType     string
	Window         Window
	WindowStart    time.Time
	MentionCount   int
	SentimentScore float64
}

// WindowStart returns the start of the window containing t, truncated the
// same way Postgres' date_trunc(unit, t) truncates it — in particular,
// Week starts on Monday (ISO 8601), matching date_trunc's behavior.
func WindowStart(window Window, t time.Time) time.Time {
	t = t.UTC()
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)

	switch window {
	case Day:
		return day
	case Week:
		offset := int(day.Weekday()) - int(time.Monday)
		if offset < 0 {
			offset += 7
		}
		return day.AddDate(0, 0, -offset)
	case Month:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	case Year:
		return time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Time{}
	}
}
