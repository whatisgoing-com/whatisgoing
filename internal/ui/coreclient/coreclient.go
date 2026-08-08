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
}

type EntityDetail struct {
	ID    int64          `json:"id"`
	Name  string         `json:"name"`
	Type  string         `json:"type"`
	Trend []EntityRollup `json:"trend"`
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
// "week", "month", "year").
func (c *Client) Trending(ctx context.Context, window string, limit int) ([]EntityRollup, error) {
	q := url.Values{"window": {window}, "limit": {strconv.Itoa(limit)}}
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
