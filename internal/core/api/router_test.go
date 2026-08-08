package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/whatisgoing-com/whatisgoing/internal/core/rollup"
	"github.com/whatisgoing-com/whatisgoing/internal/core/search"
)

type fakeRollups struct {
	top   []rollup.EntityRollup
	trend []rollup.EntityRollup
	err   error
}

func (f *fakeRollups) TopEntities(ctx context.Context, window rollup.Window, windowStart time.Time, limit int) ([]rollup.EntityRollup, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.top, nil
}

func (f *fakeRollups) ReputationTrend(ctx context.Context, entityID int64, window rollup.Window) ([]rollup.EntityRollup, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.trend, nil
}

type fakeSearcher struct {
	docs []search.Document
	err  error
}

func (f *fakeSearcher) Search(ctx context.Context, query string, limit int) ([]search.Document, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.docs, nil
}

func TestHandleTrending_DefaultsToDayWindow(t *testing.T) {
	rollups := &fakeRollups{top: []rollup.EntityRollup{
		{EntityID: 1, EntityName: "Elon Musk", EntityType: "PERSON", MentionCount: 3, SentimentScore: 0.5, WindowStart: time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)},
	}}
	router := NewRouter(rollups, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/trending", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var got []entityRollupJSON
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Elon Musk" || got[0].MentionCount != 3 {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestHandleTrending_RejectsInvalidWindow(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/trending?window=decade", nil))

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestHandleEntityTrend_ReturnsDetailAndTrend(t *testing.T) {
	rollups := &fakeRollups{trend: []rollup.EntityRollup{
		{EntityID: 42, EntityName: "Tesla", EntityType: "ORG", MentionCount: 1, SentimentScore: -0.2, WindowStart: time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)},
		{EntityID: 42, EntityName: "Tesla", EntityType: "ORG", MentionCount: 2, SentimentScore: 0.4, WindowStart: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)},
	}}
	router := NewRouter(rollups, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/entities/42", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var got entityDetailJSON
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != 42 || got.Name != "Tesla" || got.Type != "ORG" {
		t.Errorf("unexpected entity detail: %+v", got)
	}
	if len(got.Trend) != 2 {
		t.Fatalf("expected 2 trend points, got %d", len(got.Trend))
	}
}

func TestHandleEntityTrend_404sWhenNoRollupsExist(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/entities/999", nil))

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.Code)
	}
}

func TestHandleEntityTrend_RejectsNonNumericID(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/entities/not-a-number", nil))

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestHandleSearch_ReturnsResults(t *testing.T) {
	searcher := &fakeSearcher{docs: []search.Document{
		{ID: "1", URL: "https://example.com/1", Title: "Match", SourceID: "src-1", PublishedAt: time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)},
	}}
	router := NewRouter(&fakeRollups{}, searcher)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/search?q=tesla", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var got []searchResultJSON
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Match" {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestHandleSearch_RejectsMissingQuery(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/search", nil))

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestHandleHealthz(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
}
