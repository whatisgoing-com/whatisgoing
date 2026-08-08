package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/whatisgoing-com/whatisgoing/internal/core/fetcher"
)

type sourceYAML struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Type string `yaml:"type"`
}

// LoadSources reads the source list from a YAML file. See
// configs/sources.example.yaml for the schema.
func LoadSources(path string) ([]fetcher.Source, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read sources config: %w", err)
	}

	var raw struct {
		Sources []sourceYAML `yaml:"sources"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse sources config: %w", err)
	}

	sources := make([]fetcher.Source, 0, len(raw.Sources))
	for _, s := range raw.Sources {
		if s.ID == "" {
			return nil, fmt.Errorf("source %q: missing id", s.Name)
		}
		if s.URL == "" {
			return nil, fmt.Errorf("source %q: missing url", s.Name)
		}

		sourceType := fetcher.SourceType(s.Type)
		if sourceType == "" {
			sourceType = fetcher.SourceTypeRSS
		}
		if sourceType != fetcher.SourceTypeRSS && sourceType != fetcher.SourceTypeHTML {
			return nil, fmt.Errorf("source %q: unknown type %q", s.Name, s.Type)
		}

		sources = append(sources, fetcher.Source{
			ID:   s.ID,
			Name: s.Name,
			URL:  s.URL,
			Type: sourceType,
		})
	}

	return sources, nil
}
