package fetcher

import (
	"context"
	"fmt"
)

// MultiFetcher dispatches to the right Fetcher based on Source.Type.
type MultiFetcher struct {
	RSS  Fetcher
	HTML Fetcher
}

func (m *MultiFetcher) Fetch(ctx context.Context, source Source) ([]Article, error) {
	switch source.Type {
	case SourceTypeRSS, "":
		return m.RSS.Fetch(ctx, source)
	case SourceTypeHTML:
		return m.HTML.Fetch(ctx, source)
	default:
		return nil, fmt.Errorf("unknown source type %q for source %q", source.Type, source.Name)
	}
}
