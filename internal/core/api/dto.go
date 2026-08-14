package api

import (
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
)

type entityRollupJSON struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	MentionCount   int     `json:"mention_count"`
	SentimentScore float64 `json:"sentiment_score"`
	WindowStart    string  `json:"window_start"`
	PositiveCount  int     `json:"positive_count"`
	NeutralCount   int     `json:"neutral_count"`
	NegativeCount  int     `json:"negative_count"`
}

func toEntityRollupJSON(r rollup.EntityRollup) entityRollupJSON {
	return entityRollupJSON{
		ID:             r.EntityID,
		Name:           r.EntityName,
		Type:           r.EntityType,
		MentionCount:   r.MentionCount,
		SentimentScore: r.SentimentScore,
		WindowStart:    r.WindowStart.Format("2006-01-02"),
		PositiveCount:  r.PositiveCount,
		NeutralCount:   r.NeutralCount,
		NegativeCount:  r.NegativeCount,
	}
}

type sentimentBreakdownJSON struct {
	Positive int `json:"positive"`
	Neutral  int `json:"neutral"`
	Negative int `json:"negative"`
}

type sourceBreakdownJSON struct {
	SourceID     string  `json:"source_id"`
	SourceName   string  `json:"source_name"`
	MentionCount int     `json:"mention_count"`
	AvgSentiment float64 `json:"avg_sentiment"`
}

func toSourceBreakdownJSON(b rollup.SourceBreakdown) sourceBreakdownJSON {
	return sourceBreakdownJSON{
		SourceID:     b.SourceID,
		SourceName:   b.SourceName,
		MentionCount: b.MentionCount,
		AvgSentiment: b.AvgSentiment,
	}
}

type relatedEntityJSON struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	Description       string `json:"description,omitempty"`
	CooccurrenceCount int    `json:"cooccurrence_count"`
}

func toRelatedEntityJSON(re rollup.RelatedEntity) relatedEntityJSON {
	return relatedEntityJSON{
		ID:                re.ID,
		Name:              re.Name,
		Type:              re.Type,
		Description:       re.Description,
		CooccurrenceCount: re.CooccurrenceCount,
	}
}

type recentArticleJSON struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	SourceName  string `json:"source_name"`
	PublishedAt string `json:"published_at"`
}

func toRecentArticleJSON(a rollup.RecentArticle) recentArticleJSON {
	return recentArticleJSON{
		ID:          a.ID,
		Title:       a.Title,
		URL:         a.URL,
		SourceName:  a.SourceName,
		PublishedAt: a.PublishedAt.Format(time.RFC3339),
	}
}
