package search

import (
	"context"

	"github.com/meilisearch/meilisearch-go"
)

type MeiliIndexer struct {
	client meilisearch.ServiceManager
	index  string
}

func NewMeiliIndexer(url, apiKey, index string) *MeiliIndexer {
	return &MeiliIndexer{
		client: meilisearch.New(url, meilisearch.WithAPIKey(apiKey)),
		index:  index,
	}
}

func (m *MeiliIndexer) Index(ctx context.Context, doc Document) error {
	primaryKey := "id"
	_, err := m.client.Index(m.index).AddDocumentsWithContext(ctx, []Document{doc}, &meilisearch.DocumentOptions{
		PrimaryKey: &primaryKey,
	})
	return err
}
