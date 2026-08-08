package politeness

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Transport wraps an http.RoundTripper to enforce, per destination domain:
// robots.txt rules, a minimum delay between requests, and a concurrency
// cap. It's shared across all fetchers so every outbound request — RSS
// feed polls and HTML scraping alike — goes through the same limits.
type Transport struct {
	Base          http.RoundTripper
	UserAgent     string
	Robots        *RobotsChecker
	MinDelay      time.Duration
	MaxConcurrent int

	mu      sync.Mutex
	domains map[string]*domainState
}

type domainState struct {
	sem     chan struct{}
	limiter *rate.Limiter
}

func NewTransport(base http.RoundTripper, userAgent string, minDelay time.Duration, maxConcurrent int) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &Transport{
		Base:          base,
		UserAgent:     userAgent,
		Robots:        NewRobotsChecker(base, userAgent),
		MinDelay:      minDelay,
		MaxConcurrent: maxConcurrent,
		domains:       make(map[string]*domainState),
	}
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", t.UserAgent)
	}

	if req.URL.Path != "/robots.txt" {
		allowed, err := t.Robots.Allowed(req.Context(), req.URL.String())
		if err == nil && !allowed {
			return nil, fmt.Errorf("politeness: %s disallowed by robots.txt", req.URL)
		}
	}

	state := t.stateFor(req.URL.Host)

	select {
	case state.sem <- struct{}{}:
		defer func() { <-state.sem }()
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}

	if err := state.limiter.Wait(req.Context()); err != nil {
		return nil, err
	}

	return t.Base.RoundTrip(req)
}

func (t *Transport) stateFor(host string) *domainState {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.domains[host]
	if !ok {
		s = &domainState{
			sem:     make(chan struct{}, t.MaxConcurrent),
			limiter: rate.NewLimiter(rate.Every(t.MinDelay), 1),
		}
		t.domains[host] = s
	}
	return s
}
