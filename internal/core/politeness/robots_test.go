package politeness

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRobotsChecker_RespectsDisallow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nDisallow: /private\n"))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	checker := NewRobotsChecker(nil, "whatisgoingbot-test")

	allowed, err := checker.Allowed(context.Background(), srv.URL+"/private/article")
	if err != nil {
		t.Fatalf("Allowed() error = %v", err)
	}
	if allowed {
		t.Error("expected /private to be disallowed")
	}

	allowed, err = checker.Allowed(context.Background(), srv.URL+"/public/article")
	if err != nil {
		t.Fatalf("Allowed() error = %v", err)
	}
	if !allowed {
		t.Error("expected /public to be allowed")
	}
}

func TestRobotsChecker_MissingRobotsTxtAllowsEverything(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	checker := NewRobotsChecker(nil, "whatisgoingbot-test")

	allowed, err := checker.Allowed(context.Background(), srv.URL+"/anything")
	if err != nil {
		t.Fatalf("Allowed() error = %v", err)
	}
	if !allowed {
		t.Error("expected missing robots.txt to allow everything")
	}
}
