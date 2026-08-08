// Package ner is a client for the ner-sentiment service (issue #3): it
// extracts entity mentions and sentence-level sentiment from raw article
// content.
package ner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Mention is one entity occurrence returned by the ner-sentiment service:
// a typed, offset-accurate span into ExtractResult.Text, carrying the
// sentiment of the sentence it appeared in.
type Mention struct {
	Text           string  `json:"text"`
	Type           string  `json:"type"`
	Start          int     `json:"start"`
	End            int     `json:"end"`
	SentimentScore float64 `json:"sentiment_score"`
}

type ExtractResult struct {
	Title        string    `json:"title"`
	Text         string    `json:"text"`
	Entities     []Mention `json:"entities"`
	ProcessingMs float64   `json:"processing_ms"`
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

type extractRequest struct {
	HTML string `json:"html"`
	URL  string `json:"url,omitempty"`
}

// Extract calls the ner-sentiment service's POST /extract with raw
// HTML/content. url is only ever sent as an extraction hint — the service
// never fetches it itself, respecting the politeness/robots.txt boundaries
// already enforced when the content was originally fetched.
func (c *Client) Extract(ctx context.Context, html, url string) (ExtractResult, error) {
	body, err := json.Marshal(extractRequest{HTML: html, URL: url})
	if err != nil {
		return ExtractResult{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/extract", bytes.NewReader(body))
	if err != nil {
		return ExtractResult{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ExtractResult{}, fmt.Errorf("call ner-sentiment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ExtractResult{}, fmt.Errorf("ner-sentiment returned status %d", resp.StatusCode)
	}

	var result ExtractResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ExtractResult{}, fmt.Errorf("decode response: %w", err)
	}

	return result, nil
}
