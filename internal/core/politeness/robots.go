package politeness

import (
	"context"
	"net/http"
	"net/url"
	"sync"

	"github.com/temoto/robotstxt"
	"golang.org/x/sync/singleflight"
)

// RobotsChecker fetches and caches robots.txt per host for the lifetime of
// the process. It uses its own unthrottled client, deliberately separate
// from Transport, so checking robots.txt doesn't recurse back into the
// politeness rules it's used to enforce. Concurrent first-time lookups for
// the same origin are collapsed into a single fetch via singleflight, so a
// burst of requests against a never-before-seen domain doesn't each fire
// their own robots.txt request.
type RobotsChecker struct {
	client    *http.Client
	userAgent string
	group     singleflight.Group

	mu    sync.Mutex
	cache map[string]*robotstxt.RobotsData
}

func NewRobotsChecker(base http.RoundTripper, userAgent string) *RobotsChecker {
	if base == nil {
		base = http.DefaultTransport
	}
	return &RobotsChecker{
		client:    &http.Client{Transport: base},
		userAgent: userAgent,
		cache:     make(map[string]*robotstxt.RobotsData),
	}
}

func (r *RobotsChecker) Allowed(ctx context.Context, rawURL string) (bool, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false, err
	}

	data, err := r.robotsFor(ctx, u.Scheme+"://"+u.Host)
	if err != nil {
		// No robots.txt (or it failed to fetch) is conventionally treated
		// as "everything allowed", per common crawler practice.
		return true, nil
	}

	group := data.FindGroup(r.userAgent)
	path := u.Path
	if path == "" {
		path = "/"
	}
	return group.Test(path), nil
}

func (r *RobotsChecker) robotsFor(ctx context.Context, origin string) (*robotstxt.RobotsData, error) {
	r.mu.Lock()
	if data, ok := r.cache[origin]; ok {
		r.mu.Unlock()
		return data, nil
	}
	r.mu.Unlock()

	result, err, _ := r.group.Do(origin, func() (any, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/robots.txt", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", r.userAgent)

		resp, err := r.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		data, err := robotstxt.FromResponse(resp)
		if err != nil {
			return nil, err
		}

		r.mu.Lock()
		r.cache[origin] = data
		r.mu.Unlock()

		return data, nil
	})
	if err != nil {
		return nil, err
	}

	return result.(*robotstxt.RobotsData), nil
}
