package coreclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Trending_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/trending" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("window"); got != "week" {
			t.Errorf("expected window=week, got %q", got)
		}
		json.NewEncoder(w).Encode([]EntityRollup{{ID: 1, Name: "Elon Musk", Type: "PERSON", MentionCount: 5}})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	results, err := client.Trending(context.Background(), "week", 10)
	if err != nil {
		t.Fatalf("Trending() error = %v", err)
	}
	if len(results) != 1 || results[0].Name != "Elon Musk" {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestClient_EntityDetail_ReturnsFoundFalseOn404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	_, found, err := client.EntityDetail(context.Background(), 42, "day")
	if err != nil {
		t.Fatalf("EntityDetail() error = %v, want nil", err)
	}
	if found {
		t.Error("expected found=false on 404")
	}
}

func TestClient_EntityDetail_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entities/42" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(EntityDetail{
			ID: 42, Name: "Tesla", Type: "ORG",
			Trend: []EntityRollup{{WindowStart: "2026-08-07", MentionCount: 2}},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	detail, found, err := client.EntityDetail(context.Background(), 42, "day")
	if err != nil {
		t.Fatalf("EntityDetail() error = %v", err)
	}
	if !found || detail.Name != "Tesla" || len(detail.Trend) != 1 {
		t.Errorf("unexpected detail: %+v (found=%v)", detail, found)
	}
}

func TestClient_Search_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "tesla" {
			t.Errorf("expected q=tesla, got %q", got)
		}
		json.NewEncoder(w).Encode([]SearchResult{{ID: "1", Title: "Match"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	results, err := client.Search(context.Background(), "tesla", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 1 || results[0].Title != "Match" {
		t.Errorf("unexpected results: %+v", results)
	}
}
