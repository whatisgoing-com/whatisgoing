package wikidata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func fakeServer(t *testing.T, results []map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"search": results})
	}))
}

func TestClient_Search_NotFoundWhenTopResultIsGeneric(t *testing.T) {
	// Real top result from the live Wikidata API for a bare "Trump"
	// search (2026-08-14). Search must not scan past this looking for a
	// better candidate — tried that, and the next "non-generic" result
	// was an obscure open star cluster matched via an alias, which is
	// actively wrong, not just a weaker guess (see Search's doc comment).
	server := fakeServer(t, []map[string]string{
		{"id": "Q16944413", "label": "Trump", "description": "family name"},
		{"id": "Q11387", "label": "open cluster", "description": "group of \"sibling\" stars"},
	})
	defer server.Close()

	client := NewClient(server.URL, nil)
	_, found, err := client.Search(context.Background(), "Trump")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if found {
		t.Error("expected no match when the top result is generic, even though a later one isn't")
	}
}

func TestClient_Search_ReturnsGoodMatchDirectly(t *testing.T) {
	// Real response shape for "Donald Trump" (2026-08-14): the correct
	// entity is the top result, no filtering needed.
	server := fakeServer(t, []map[string]string{
		{"id": "Q22686", "label": "Donald Trump", "description": "45th and 47th President of the United States (2017-2021; since 2025)"},
	})
	defer server.Close()

	client := NewClient(server.URL, nil)
	match, found, err := client.Search(context.Background(), "Donald Trump")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if !found || match.ID != "Q22686" {
		t.Errorf("expected Q22686 Donald Trump, got found=%v match=%+v", found, match)
	}
}

func TestClient_Search_NotFoundOnEmptyResults(t *testing.T) {
	server := fakeServer(t, []map[string]string{})
	defer server.Close()

	client := NewClient(server.URL, nil)
	_, found, err := client.Search(context.Background(), "asdkjhaskjdh")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if found {
		t.Error("expected no match for empty results")
	}
}

func TestIsGeneric(t *testing.T) {
	tests := []struct {
		desc string
		want bool
	}{
		{"family name", true},
		{"Family Name", true}, // case-insensitive
		{"musical instrument", true},
		{"disambiguation page", true},
		{"45th and 47th President of the United States", false},
		{"American multinational technology company", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isGeneric(tt.desc); got != tt.want {
			t.Errorf("isGeneric(%q) = %v, want %v", tt.desc, got, tt.want)
		}
	}
}
