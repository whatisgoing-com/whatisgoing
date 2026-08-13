// Package wikipedia is a minimal client for Wikipedia's public REST
// summary API, used by cmd/entity-resolver (issue #26) to give a
// resolved entity a short descriptive paragraph — separate from
// internal/core/wikidata, which resolves identity (which real-world
// entity a mention refers to) via a different API on a different host.
package wikipedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// DefaultBaseURL is Wikipedia's real public REST summary endpoint.
const DefaultBaseURL = "https://en.wikipedia.org/api/rest_v1/page/summary"

type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a client against baseURL (pass DefaultBaseURL in
// production; tests inject an httptest server URL instead).
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, baseURL: baseURL}
}

// Summary returns a short descriptive paragraph for the Wikipedia page
// matching title (typically a Wikidata match's canonical label, e.g.
// "Donald Trump" or "Boeing" — Wikipedia and Wikidata's labels usually
// coincide for well-known entities), or found=false if there's no such
// page.
func (c *Client) Summary(ctx context.Context, title string) (string, bool, error) {
	reqURL := c.baseURL + "/" + url.PathEscape(strings.ReplaceAll(title, " ", "_"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "whatisgoingbot/0.1 (+https://whatisgoing.com; contact: hello@whatisgoing.com)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("call wikipedia: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("wikipedia returned status %d", resp.StatusCode)
	}

	var body struct {
		Extract string `json:"extract"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", false, fmt.Errorf("decode wikipedia response: %w", err)
	}
	if body.Extract == "" {
		return "", false, nil
	}
	return body.Extract, true, nil
}
