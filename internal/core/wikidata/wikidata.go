// Package wikidata is a minimal client for Wikidata's public search API,
// used by cmd/entity-resolver (issue #26) to canonicalize entity name
// variants: two mentions that resolve to the same Wikidata item (QID)
// are the same real-world entity, whatever surface text NER extracted.
package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// DefaultBaseURL is Wikidata's real public API endpoint.
const DefaultBaseURL = "https://www.wikidata.org/w/api.php"

// Match is one Wikidata search result: a candidate real-world entity,
// with its canonical label (the name entities get renamed to once
// resolved) and a short description, used to filter out generic
// non-entity results (see Search).
type Match struct {
	ID          string // QID, e.g. "Q22686"
	Label       string
	Description string
}

// genericDescriptions are substrings seen in real Wikidata responses for
// items that are not the specific real-world entity a mention refers to
// — dictionary/meta entries (e.g. searching "Trump" alone surfaces "Trump
// (family name)" as its top hit, confirmed against the live API). Search
// only ever looks at the top result (see its comment for why) — this
// list decides whether that top result gets trusted at all.
var genericDescriptions = []string{
	"family name",
	"given name",
	"surname",
	"disambiguation page",
	"wikimedia disambiguation page",
	"musical instrument",
	"playing card",
	"topic album",
}

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

// Search returns Wikidata's top search result for name, or found=false if
// there's no result or the top one is filtered as generic (see
// genericDescriptions). Deliberately only ever looks at the top result,
// never scans past it looking for something better: tried that first and
// it's actively unsafe, not just noisier — searching bare "Trump", after
// skipping the literal "family name" entry, surfaced an obscure open star
// cluster (matched via an alias) as the next "non-generic" candidate,
// confirmed against the live API. A search engine's 2nd/3rd-ranked result
// isn't a weaker signal of the right answer, it's evidence there wasn't
// one — cmd/entity-resolver's local alias-matching fallback is what's
// meant to catch those cases instead of trusting a bad ranked guess.
func (c *Client) Search(ctx context.Context, name string) (Match, bool, error) {
	q := url.Values{
		"action":   {"wbsearchentities"},
		"search":   {name},
		"language": {"en"},
		"type":     {"item"},
		"format":   {"json"},
		"limit":    {"1"},
	}
	reqURL := c.baseURL + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return Match{}, false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "whatisgoingbot/0.1 (+https://whatisgoing.com; contact: hello@whatisgoing.com)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Match{}, false, fmt.Errorf("call wikidata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Match{}, false, fmt.Errorf("wikidata returned status %d", resp.StatusCode)
	}

	var body struct {
		Search []struct {
			ID          string `json:"id"`
			Label       string `json:"label"`
			Description string `json:"description"`
		} `json:"search"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Match{}, false, fmt.Errorf("decode wikidata response: %w", err)
	}

	if len(body.Search) == 0 {
		return Match{}, false, nil
	}
	top := body.Search[0]
	if isGeneric(top.Description) {
		return Match{}, false, nil
	}
	return Match{ID: top.ID, Label: top.Label, Description: top.Description}, true, nil
}

func isGeneric(description string) bool {
	desc := strings.ToLower(description)
	for _, needle := range genericDescriptions {
		if strings.Contains(desc, needle) {
			return true
		}
	}
	return false
}
