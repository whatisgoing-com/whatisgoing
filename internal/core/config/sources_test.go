package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sources.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSources_ParsesTypesAndDefaultsToRSS(t *testing.T) {
	path := writeTempConfig(t, `
sources:
  - id: example-rss
    name: Example RSS
    url: https://example.com/rss.xml
  - id: example-html
    name: Example HTML
    url: https://example.com/news
    type: html
`)

	sources, err := LoadSources(path)
	if err != nil {
		t.Fatalf("LoadSources() error = %v", err)
	}

	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].Type != fetcher.SourceTypeRSS {
		t.Errorf("expected default type rss, got %q", sources[0].Type)
	}
	if sources[1].Type != fetcher.SourceTypeHTML {
		t.Errorf("expected type html, got %q", sources[1].Type)
	}
}

func TestLoadSources_RejectsMissingFields(t *testing.T) {
	cases := []string{
		"sources:\n  - name: Missing ID\n    url: https://example.com/rss.xml\n",
		"sources:\n  - id: missing-url\n    name: Missing URL\n",
		"sources:\n  - id: bad-type\n    name: Bad Type\n    url: https://example.com\n    type: carrier-pigeon\n",
	}

	for _, c := range cases {
		path := writeTempConfig(t, c)
		if _, err := LoadSources(path); err == nil {
			t.Errorf("expected error for config: %s", c)
		}
	}
}

func TestLoadSources_MissingFile(t *testing.T) {
	if _, err := LoadSources("/nonexistent/sources.yaml"); err == nil {
		t.Fatal("expected error for missing file")
	}
}
