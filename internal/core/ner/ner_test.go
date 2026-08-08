package ner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Extract_SendsRequestAndParsesResponse(t *testing.T) {
	var gotReq extractRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/extract" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		resp := ExtractResult{
			Title: "Title",
			Text:  "Elon Musk announced a product.",
			Entities: []Mention{
				{Text: "Elon Musk", Type: "PERSON", Start: 0, End: 9, SentimentScore: 0.8},
			},
			ProcessingMs: 12.3,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	result, err := client.Extract(context.Background(), "<p>Elon Musk announced a product.</p>", "https://example.com/a")
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	if gotReq.HTML != "<p>Elon Musk announced a product.</p>" || gotReq.URL != "https://example.com/a" {
		t.Fatalf("unexpected request body sent: %+v", gotReq)
	}

	if len(result.Entities) != 1 || result.Entities[0].Text != "Elon Musk" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestClient_Extract_ReturnsErrorOnNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	if _, err := client.Extract(context.Background(), "<p>x</p>", ""); err == nil {
		t.Fatal("expected error on non-200 response")
	}
}
