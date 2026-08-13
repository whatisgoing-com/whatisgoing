package wikipedia

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Summary_ReturnsExtract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Donald_Trump" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"extract": "Donald John Trump is an American politician, media personality, and businessman.",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	extract, found, err := client.Summary(context.Background(), "Donald Trump")
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if !found || extract == "" {
		t.Fatalf("expected a non-empty extract, got found=%v extract=%q", found, extract)
	}
}

func TestClient_Summary_NotFoundOn404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	_, found, err := client.Summary(context.Background(), "Some Nonexistent Page")
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if found {
		t.Error("expected found=false on 404")
	}
}

func TestClient_Summary_NotFoundOnEmptyExtract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"extract": ""})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	_, found, err := client.Summary(context.Background(), "Empty Extract Page")
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if found {
		t.Error("expected found=false when extract is empty")
	}
}

func TestClient_Summary_EscapesTitleForURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// net/http decodes percent-escapes into r.URL.Path for routing —
		// the meaningful assertion is that the request line itself was
		// escaped (EscapedPath), and the title round-trips correctly.
		if r.URL.EscapedPath() != "/O%27Brien" {
			t.Fatalf("expected escaped request path /O%%27Brien, got %s", r.URL.EscapedPath())
		}
		if r.URL.Path != "/O'Brien" {
			t.Fatalf("expected decoded path /O'Brien, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{"extract": "..."})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	if _, _, err := client.Summary(context.Background(), "O'Brien"); err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
}
