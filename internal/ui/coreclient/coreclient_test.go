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
		if got := r.URL.Query().Get("type"); got != "" {
			t.Errorf("expected no type param, got %q", got)
		}
		json.NewEncoder(w).Encode([]EntityRollup{{ID: 1, Name: "Elon Musk", Type: "PERSON", MentionCount: 5}})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	results, err := client.Trending(context.Background(), "week", "", 10)
	if err != nil {
		t.Fatalf("Trending() error = %v", err)
	}
	if len(results) != 1 || results[0].Name != "Elon Musk" {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestClient_Trending_SetsTypeWhenGiven(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("type"); got != "ORG" {
			t.Errorf("expected type=ORG, got %q", got)
		}
		json.NewEncoder(w).Encode([]EntityRollup{})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	if _, err := client.Trending(context.Background(), "day", "ORG", 10); err != nil {
		t.Fatalf("Trending() error = %v", err)
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

func TestClient_OverallTrend_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/trend/overall" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]OverallTrendPoint{{WindowStart: "2026-08-08", TotalMentions: 12, AvgSentiment: 0.3}})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	points, err := client.OverallTrend(context.Background(), "day", 30)
	if err != nil {
		t.Fatalf("OverallTrend() error = %v", err)
	}
	if len(points) != 1 || points[0].TotalMentions != 12 {
		t.Errorf("unexpected points: %+v", points)
	}
}

func TestClient_SentimentBreakdown_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/sentiment" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("window"); got != "day" {
			t.Errorf("expected window=day, got %q", got)
		}
		json.NewEncoder(w).Encode(SentimentBreakdown{Positive: 5, Neutral: 2, Negative: 3})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	breakdown, err := client.SentimentBreakdown(context.Background(), "day")
	if err != nil {
		t.Fatalf("SentimentBreakdown() error = %v", err)
	}
	if breakdown.Positive != 5 || breakdown.Neutral != 2 || breakdown.Negative != 3 {
		t.Errorf("unexpected breakdown: %+v", breakdown)
	}
}

func TestClient_WindowStats_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/stats" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("window"); got != "week" {
			t.Errorf("expected window=week, got %q", got)
		}
		json.NewEncoder(w).Encode(WindowStats{ArticleCount: 42, EntityCount: 17, WindowStart: "2026-08-10", WindowEnd: "2026-08-17"})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	stats, err := client.WindowStats(context.Background(), "week")
	if err != nil {
		t.Fatalf("WindowStats() error = %v", err)
	}
	if stats.ArticleCount != 42 || stats.EntityCount != 17 || stats.WindowStart != "2026-08-10" || stats.WindowEnd != "2026-08-17" {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestClient_SearchEntities_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); got != "musk" {
			t.Errorf("expected q=musk, got %q", got)
		}
		json.NewEncoder(w).Encode([]EntitySummary{{ID: 7, Name: "Elon Musk", Type: "PERSON"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	results, err := client.SearchEntities(context.Background(), "musk", 10)
	if err != nil {
		t.Fatalf("SearchEntities() error = %v", err)
	}
	if len(results) != 1 || results[0].Name != "Elon Musk" {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestClient_SourceBreakdown_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entities/42/sources" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]SourceBreakdown{{SourceID: "bbc-world", SourceName: "BBC World News", MentionCount: 5, AvgSentiment: -0.3}})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	results, err := client.SourceBreakdown(context.Background(), 42)
	if err != nil {
		t.Fatalf("SourceBreakdown() error = %v", err)
	}
	if len(results) != 1 || results[0].SourceName != "BBC World News" {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestClient_RecentArticles_OmitsEntityIDWhenZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/articles/recent" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Has("entity_id") {
			t.Errorf("expected no entity_id param for entityID=0, got %q", r.URL.Query().Get("entity_id"))
		}
		json.NewEncoder(w).Encode([]RecentArticle{{ID: 1, Title: "Headline", SourceName: "BBC World News"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	results, err := client.RecentArticles(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("RecentArticles() error = %v", err)
	}
	if len(results) != 1 || results[0].Title != "Headline" {
		t.Errorf("unexpected results: %+v", results)
	}
}

func TestClient_RecentArticles_SetsEntityIDWhenNonZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("entity_id"); got != "42" {
			t.Errorf("expected entity_id=42, got %q", got)
		}
		json.NewEncoder(w).Encode([]RecentArticle{})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	if _, err := client.RecentArticles(context.Background(), 42, 10); err != nil {
		t.Fatalf("RecentArticles() error = %v", err)
	}
}

func TestClient_RelatedEntities_ParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entities/42/related" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]RelatedEntity{{ID: 7, Name: "Sam Altman", Type: "PERSON", CooccurrenceCount: 4}})
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	results, err := client.RelatedEntities(context.Background(), 42, 10)
	if err != nil {
		t.Fatalf("RelatedEntities() error = %v", err)
	}
	if len(results) != 1 || results[0].Name != "Sam Altman" || results[0].CooccurrenceCount != 4 {
		t.Errorf("unexpected results: %+v", results)
	}
}
