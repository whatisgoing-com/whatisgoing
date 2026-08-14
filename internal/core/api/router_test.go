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
	top             []rollup.EntityRollup
	topByType       []rollup.EntityRollup
	trend           []rollup.EntityRollup
	overall         []rollup.OverallTrendPoint
	entitySearch    []rollup.EntitySummary
	sentiment       rollup.SentimentBreakdown
	sourceBreakdown []rollup.SourceBreakdown
	recentArticles  []rollup.RecentArticle
	relatedEntities []rollup.RelatedEntity
	err             error
}

func (f *fakeRollups) TopEntities(ctx context.Context, window rollup.Window, windowStart time.Time, limit int) ([]rollup.EntityRollup, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.top, nil
}

func (f *fakeRollups) TopEntitiesByType(ctx context.Context, window rollup.Window, windowStart time.Time, entityType string, limit int) ([]rollup.EntityRollup, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.topByType, nil
}

func (f *fakeRollups) ReputationTrend(ctx context.Context, entityID int64, window rollup.Window) ([]rollup.EntityRollup, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.trend, nil
}

func (f *fakeRollups) OverallTrend(ctx context.Context, window rollup.Window, limit int) ([]rollup.OverallTrendPoint, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.overall, nil
}

func (f *fakeRollups) SearchEntities(ctx context.Context, query string, limit int) ([]rollup.EntitySummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.entitySearch, nil
}

func (f *fakeRollups) SentimentBreakdown(ctx context.Context, window rollup.Window, windowStart time.Time) (rollup.SentimentBreakdown, error) {
	if f.err != nil {
		return rollup.SentimentBreakdown{}, f.err
	}
	return f.sentiment, nil
}

func (f *fakeRollups) SourceBreakdown(ctx context.Context, entityID int64) ([]rollup.SourceBreakdown, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.sourceBreakdown, nil
}

func (f *fakeRollups) RecentArticles(ctx context.Context, entityID int64, limit int) ([]rollup.RecentArticle, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.recentArticles, nil
}

func (f *fakeRollups) RelatedEntities(ctx context.Context, entityID int64, limit int) ([]rollup.RelatedEntity, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.relatedEntities, nil
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

func TestHandleTrending_FiltersByType(t *testing.T) {
	rollups := &fakeRollups{
		top:       []rollup.EntityRollup{{EntityID: 1, EntityName: "mixed", EntityType: "PERSON"}},
		topByType: []rollup.EntityRollup{{EntityID: 2, EntityName: "OpenAI", EntityType: "ORG", MentionCount: 7}},
	}
	router := NewRouter(rollups, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/trending?type=ORG", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var got []entityRollupJSON
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Name != "OpenAI" || got[0].MentionCount != 7 {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestHandleTrending_RejectsInvalidType(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/trending?type=PLANET", nil))

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
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

func TestHandleOverallTrend_ReturnsPoints(t *testing.T) {
	rollups := &fakeRollups{overall: []rollup.OverallTrendPoint{
		{WindowStart: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC), TotalMentions: 10, AvgSentiment: 0.2},
		{WindowStart: time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC), TotalMentions: 15, AvgSentiment: -0.1},
	}}
	router := NewRouter(rollups, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/trend/overall?window=day", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var got []overallTrendPointJSON
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 || got[0].TotalMentions != 10 || got[1].AvgSentiment != -0.1 {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestHandleOverallTrend_RejectsInvalidWindow(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/trend/overall?window=decade", nil))

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestHandleEntitySearch_ReturnsMatches(t *testing.T) {
	rollups := &fakeRollups{entitySearch: []rollup.EntitySummary{{ID: 7, Name: "Elon Musk", Type: "PERSON"}}}
	router := NewRouter(rollups, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/entities?q=musk", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var got []entitySummaryJSON
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Elon Musk" {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestHandleEntitySearch_RejectsMissingQuery(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/entities", nil))

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestHandleSentimentBreakdown_ReturnsCounts(t *testing.T) {
	rollups := &fakeRollups{sentiment: rollup.SentimentBreakdown{Positive: 5, Neutral: 2, Negative: 3}}
	router := NewRouter(rollups, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/sentiment?window=day", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var got sentimentBreakdownJSON
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Positive != 5 || got.Neutral != 2 || got.Negative != 3 {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestHandleSentimentBreakdown_RejectsInvalidWindow(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/sentiment?window=decade", nil))

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestHandleSourceBreakdown_ReturnsResults(t *testing.T) {
	rollups := &fakeRollups{sourceBreakdown: []rollup.SourceBreakdown{
		{SourceID: "bbc-world", SourceName: "BBC World News", MentionCount: 5, AvgSentiment: -0.3},
	}}
	router := NewRouter(rollups, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/entities/42/sources", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var got []sourceBreakdownJSON
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].SourceName != "BBC World News" || got[0].MentionCount != 5 {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestHandleSourceBreakdown_RejectsNonNumericID(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/entities/not-a-number/sources", nil))

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestHandleRecentArticles_ReturnsResults(t *testing.T) {
	rollups := &fakeRollups{recentArticles: []rollup.RecentArticle{
		{ID: 1, Title: "Headline", URL: "https://example.com/1", SourceName: "BBC World News", PublishedAt: time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)},
	}}
	router := NewRouter(rollups, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/articles/recent", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var got []recentArticleJSON
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Title != "Headline" {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestHandleRecentArticles_RejectsInvalidEntityID(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/articles/recent?entity_id=not-a-number", nil))

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestHandleRelatedEntities_ReturnsResults(t *testing.T) {
	rollups := &fakeRollups{relatedEntities: []rollup.RelatedEntity{
		{ID: 7, Name: "Sam Altman", Type: "PERSON", CooccurrenceCount: 4},
	}}
	router := NewRouter(rollups, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/entities/42/related", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}

	var got []relatedEntityJSON
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Sam Altman" || got[0].CooccurrenceCount != 4 {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestHandleRelatedEntities_RejectsNonNumericID(t *testing.T) {
	router := NewRouter(&fakeRollups{}, &fakeSearcher{})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/entities/not-a-number/related", nil))

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
