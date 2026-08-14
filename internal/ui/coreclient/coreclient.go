// Package coreclient is the UI/BFF service's (issue #6) HTTP client for
// core's internal JSON API (internal/core/api): trending entities,
// per-entity reputation trend, and search.
package coreclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type EntityRollup struct {
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

type EntityDetail struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Trend       []EntityRollup `json:"trend"`
}

type SearchResult struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	SourceID    string `json:"source_id"`
	PublishedAt string `json:"published_at"`
}

type OverallTrendPoint struct {
	WindowStart   string  `json:"window_start"`
	TotalMentions int     `json:"total_mentions"`
	AvgSentiment  float64 `json:"avg_sentiment"`
}

type EntitySummary struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type SentimentBreakdown struct {
	Positive int `json:"positive"`
	Neutral  int `json:"neutral"`
	Negative int `json:"negative"`
}

type WindowStats struct {
	ArticleCount int    `json:"article_count"`
	EntityCount  int    `json:"entity_count"`
	WindowStart  string `json:"window_start"`
	WindowEnd    string `json:"window_end"`
}

type SourceBreakdown struct {
	SourceID     string  `json:"source_id"`
	SourceName   string  `json:"source_name"`
	MentionCount int     `json:"mention_count"`
	AvgSentiment float64 `json:"avg_sentiment"`
}

type RecentArticle struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	SourceName  string `json:"source_name"`
	PublishedAt string `json:"published_at"`
}

type RelatedEntity struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	Description       string `json:"description"`
	CooccurrenceCount int    `json:"cooccurrence_count"`
}

type Client struct {
	httpClient *http.Client
	baseURL    string
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, baseURL: baseURL}
}

// Trending returns the most-mentioned entities for a window ("day",
// "week", "month", "year"), optionally scoped to one entity type
// (PERSON/ORG/EVENT) — pass "" for the unscoped, mixed-type ranking.
func (c *Client) Trending(ctx context.Context, window, entityType string, limit int) ([]EntityRollup, error) {
	q := url.Values{"window": {window}, "limit": {strconv.Itoa(limit)}}
	if entityType != "" {
		q.Set("type", entityType)
	}
	var out []EntityRollup
	if err := c.get(ctx, "/api/trending?"+q.Encode(), &out); err != nil {
		return nil, fmt.Errorf("get trending: %w", err)
	}
	return out, nil
}

// EntityDetail returns an entity's detail and reputation trend. found is
// false (with a nil error) if core reports the entity doesn't exist or
// has no rollups yet.
func (c *Client) EntityDetail(ctx context.Context, id int64, window string) (detail EntityDetail, found bool, err error) {
	q := url.Values{"window": {window}}
	path := fmt.Sprintf("/api/entities/%d?%s", id, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return EntityDetail{}, false, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return EntityDetail{}, false, fmt.Errorf("get entity detail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return EntityDetail{}, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return EntityDetail{}, false, fmt.Errorf("get entity detail: unexpected status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return EntityDetail{}, false, fmt.Errorf("decode entity detail: %w", err)
	}
	return detail, true, nil
}

// Search runs a full-text search against indexed articles.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	q := url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}}
	var out []SearchResult
	if err := c.get(ctx, "/api/search?"+q.Encode(), &out); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return out, nil
}

// OverallTrend returns the aggregate mentions/sentiment time series across
// every entity, for the home dashboard's chart.
func (c *Client) OverallTrend(ctx context.Context, window string, limit int) ([]OverallTrendPoint, error) {
	q := url.Values{"window": {window}, "limit": {strconv.Itoa(limit)}}
	var out []OverallTrendPoint
	if err := c.get(ctx, "/api/trend/overall?"+q.Encode(), &out); err != nil {
		return nil, fmt.Errorf("get overall trend: %w", err)
	}
	return out, nil
}

// SearchEntities finds entities by name, for the dashboard's entity
// search bar.
func (c *Client) SearchEntities(ctx context.Context, query string, limit int) ([]EntitySummary, error) {
	q := url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}}
	var out []EntitySummary
	if err := c.get(ctx, "/api/entities?"+q.Encode(), &out); err != nil {
		return nil, fmt.Errorf("search entities: %w", err)
	}
	return out, nil
}

// SentimentBreakdown returns the real positive/neutral/negative mention
// count breakdown across every entity for the current window bucket, for
// the dashboard's sentiment pie chart.
func (c *Client) SentimentBreakdown(ctx context.Context, window string) (SentimentBreakdown, error) {
	q := url.Values{"window": {window}}
	var out SentimentBreakdown
	if err := c.get(ctx, "/api/sentiment?"+q.Encode(), &out); err != nil {
		return SentimentBreakdown{}, fmt.Errorf("get sentiment breakdown: %w", err)
	}
	return out, nil
}

// WindowStats returns article/entity counts plus the real date range
// covered by the selected window, for the home page's stat tiles and
// window-range label.
func (c *Client) WindowStats(ctx context.Context, window string) (WindowStats, error) {
	q := url.Values{"window": {window}}
	var out WindowStats
	if err := c.get(ctx, "/api/stats?"+q.Encode(), &out); err != nil {
		return WindowStats{}, fmt.Errorf("get window stats: %w", err)
	}
	return out, nil
}

// SourceBreakdown returns one entity's mention count + average sentiment
// per source, for the entity detail page's by-source breakdown.
func (c *Client) SourceBreakdown(ctx context.Context, entityID int64) ([]SourceBreakdown, error) {
	var out []SourceBreakdown
	if err := c.get(ctx, fmt.Sprintf("/api/entities/%d/sources", entityID), &out); err != nil {
		return nil, fmt.Errorf("get source breakdown: %w", err)
	}
	return out, nil
}

// RecentArticles returns the most recently published articles, newest
// first, for the recent-articles list. entityID == 0 means unfiltered
// (the home page); a non-zero entityID scopes it to one entity's page.
func (c *Client) RecentArticles(ctx context.Context, entityID int64, limit int) ([]RecentArticle, error) {
	q := url.Values{"limit": {strconv.Itoa(limit)}}
	if entityID != 0 {
		q.Set("entity_id", strconv.FormatInt(entityID, 10))
	}
	var out []RecentArticle
	if err := c.get(ctx, "/api/articles/recent?"+q.Encode(), &out); err != nil {
		return nil, fmt.Errorf("get recent articles: %w", err)
	}
	return out, nil
}

// RelatedEntities returns the entities that co-occurred most often with
// entityID across articles, ranked by shared-article count descending.
func (c *Client) RelatedEntities(ctx context.Context, entityID int64, limit int) ([]RelatedEntity, error) {
	q := url.Values{"limit": {strconv.Itoa(limit)}}
	var out []RelatedEntity
	if err := c.get(ctx, fmt.Sprintf("/api/entities/%d/related?%s", entityID, q.Encode()), &out); err != nil {
		return nil, fmt.Errorf("get related entities: %w", err)
	}
	return out, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}
