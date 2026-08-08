package api

import "github.com/whatisgoing-com/whatisgoing/internal/core/rollup"

type entityRollupJSON struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	MentionCount   int     `json:"mention_count"`
	SentimentScore float64 `json:"sentiment_score"`
	WindowStart    string  `json:"window_start"`
}

func toEntityRollupJSON(r rollup.EntityRollup) entityRollupJSON {
	return entityRollupJSON{
		ID:             r.EntityID,
		Name:           r.EntityName,
		Type:           r.EntityType,
		MentionCount:   r.MentionCount,
		SentimentScore: r.SentimentScore,
		WindowStart:    r.WindowStart.Format("2006-01-02"),
	}
}
