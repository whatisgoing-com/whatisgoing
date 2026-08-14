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

// ParseWindow validates s against the known Window values.
func ParseWindow(s string) (Window, bool) {
	w := Window(s)
	for _, valid := range Windows {
		if w == valid {
			return w, true
		}
	}
	return "", false
}

// EntityRollup is one (entity, window, window start) aggregate: how many
// times the entity was mentioned in that window, and its average
// sentiment.
type EntityRollup struct {
	EntityID          int64
	EntityName        string
	EntityType        string
	EntityDescription string // short Wikipedia summary, "" if not yet resolved (issue #26)
	Window            Window
	WindowStart       time.Time
	MentionCount      int
	SentimentScore    float64
	PositiveCount     int
	NeutralCount      int
	NegativeCount     int
}

// OverallTrendPoint is one window_start's aggregate across every entity:
// total mentions and average sentiment, for the home dashboard's
// time-series chart (issue #19) — not per-entity, unlike EntityRollup.
type OverallTrendPoint struct {
	WindowStart   time.Time
	TotalMentions int
	AvgSentiment  float64
}

// EntitySummary is a lightweight entity identity, returned by entity name
// search (issue #19) — no rollup data attached.
type EntitySummary struct {
	ID   int64
	Name string
	Type string
}

// SentimentBreakdown is positive/neutral/negative mention counts summed
// across every entity for one window_start — the home dashboard's overall
// sentiment pie chart (issue #21).
type SentimentBreakdown struct {
	Positive int
	Neutral  int
	Negative int
}

// SourceBreakdown is one source's mention count + average sentiment for a
// single entity, across all time — the entity detail page's by-source
// breakdown (issue #24): which outlets cover this entity, and how
// differently they cover it.
type SourceBreakdown struct {
	SourceID     string
	SourceName   string
	MentionCount int
	AvgSentiment float64
}

// RelatedEntity is another entity that co-occurred with a given entity in
// at least one article, ranked by how many articles they shared — the
// entity detail page's "related entities" section (issue #32). Backed by
// entity_cooccurrence, which is populated on ingest but had no reader
// until this feature.
type RelatedEntity struct {
	ID                int64
	Name              string
	Type              string
	Description       string
	CooccurrenceCount int
}

// WindowStats is the count of distinct articles published and distinct
// entities mentioned within a window bucket — the home page's "Articles"
// and "Entities mentioned" stat tiles (issue #37).
type WindowStats struct {
	ArticleCount int
	EntityCount  int
}

// RecentArticle is a lightweight article summary — headline, source,
// link, publish time — for the recent-articles list (issue #24). No body
// content: the dashboard links out to the original rather than
// reproducing it.
type RecentArticle struct {
	ID          int64
	Title       string
	URL         string
	SourceName  string
	PublishedAt time.Time
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

// WindowEnd returns the exclusive end of the window that windowStart (as
// returned by WindowStart) begins — the day after, the Monday after next,
// the 1st of next month, or Jan 1 of next year.
func WindowEnd(window Window, windowStart time.Time) time.Time {
	switch window {
	case Day:
		return windowStart.AddDate(0, 0, 1)
	case Week:
		return windowStart.AddDate(0, 0, 7)
	case Month:
		return windowStart.AddDate(0, 1, 0)
	case Year:
		return windowStart.AddDate(1, 0, 0)
	default:
		return windowStart
	}
}
