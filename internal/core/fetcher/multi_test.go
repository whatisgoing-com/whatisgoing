package fetcher

import (
	"context"
	"testing"
)

type stubFetcher struct {
	called bool
}

func (s *stubFetcher) Fetch(ctx context.Context, source Source) ([]Article, error) {
	s.called = true
	return nil, nil
}

func TestMultiFetcher_DispatchesByType(t *testing.T) {
	cases := []struct {
		name       string
		sourceType SourceType
		wantRSS    bool
	}{
		{"explicit rss", SourceTypeRSS, true},
		{"empty type defaults to rss", "", true},
		{"html", SourceTypeHTML, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rss := &stubFetcher{}
			htmlF := &stubFetcher{}
			m := &MultiFetcher{RSS: rss, HTML: htmlF}

			if _, err := m.Fetch(context.Background(), Source{Type: tc.sourceType}); err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}

			if rss.called != tc.wantRSS {
				t.Errorf("rss.called = %v, want %v", rss.called, tc.wantRSS)
			}
			if htmlF.called == tc.wantRSS {
				t.Errorf("html.called = %v, want %v", htmlF.called, !tc.wantRSS)
			}
		})
	}
}

func TestMultiFetcher_UnknownTypeErrors(t *testing.T) {
	m := &MultiFetcher{RSS: &stubFetcher{}, HTML: &stubFetcher{}}
	if _, err := m.Fetch(context.Background(), Source{Type: "carrier-pigeon"}); err == nil {
		t.Fatal("expected error for unknown source type")
	}
}
