package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// HTMLFetcher discovers article links from a listing page and fetches each
// one's raw HTML. It's the fallback for sources without an RSS feed.
// Cleaning/extraction of that HTML into article text happens in the
// ner-sentiment service (issue #3) — this fetcher only discovers and
// retrieves.
type HTMLFetcher struct {
	Client   *http.Client
	MaxLinks int
}

func NewHTMLFetcher(client *http.Client, maxLinks int) *HTMLFetcher {
	if client == nil {
		client = http.DefaultClient
	}
	if maxLinks <= 0 {
		maxLinks = 25
	}
	return &HTMLFetcher{Client: client, MaxLinks: maxLinks}
}

func (f *HTMLFetcher) Fetch(ctx context.Context, source Source) ([]Article, error) {
	listingBody, err := f.get(ctx, source.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch listing page: %w", err)
	}

	links, err := discoverLinks(listingBody, source.URL)
	if err != nil {
		return nil, fmt.Errorf("discover links: %w", err)
	}
	if len(links) > f.MaxLinks {
		links = links[:f.MaxLinks]
	}

	articles := make([]Article, 0, len(links))
	for _, link := range links {
		body, err := f.get(ctx, link)
		if err != nil {
			// Skip pages that fail to fetch rather than failing the whole
			// source over one bad link.
			continue
		}

		articles = append(articles, Article{
			SourceID: source.ID,
			URL:      link,
			Title:    extractTitle(body),
			Content:  body,
			DedupKey: link,
		})
	}

	return articles, nil
}

func (f *HTMLFetcher) get(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d for %s", resp.StatusCode, rawURL)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// discoverLinks returns the unique, same-host, absolute links found in the
// given HTML, in document order.
func discoverLinks(rawHTML, pageURL string) ([]string, error) {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil, err
	}

	tokenizer := html.NewTokenizer(strings.NewReader(rawHTML))
	seen := make(map[string]bool)
	var links []string

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}

		token := tokenizer.Token()
		if token.Data != "a" {
			continue
		}

		for _, attr := range token.Attr {
			if attr.Key != "href" {
				continue
			}

			resolved, err := base.Parse(attr.Val)
			if err != nil || resolved.Host != base.Host {
				continue
			}
			resolved.Fragment = ""

			link := resolved.String()
			if !seen[link] {
				seen[link] = true
				links = append(links, link)
			}
		}
	}

	return links, nil
}

func extractTitle(rawHTML string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(rawHTML))
	inTitle := false

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			return ""
		}

		switch tt {
		case html.StartTagToken:
			if tokenizer.Token().Data == "title" {
				inTitle = true
			}
		case html.TextToken:
			if inTitle {
				return strings.TrimSpace(string(tokenizer.Text()))
			}
		case html.EndTagToken:
			if tokenizer.Token().Data == "title" {
				return ""
			}
		}
	}
}
