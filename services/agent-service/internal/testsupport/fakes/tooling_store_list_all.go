package fakes

import (
	"context"
	"sort"

	"agent-platform/services/agent-service/internal/core/domain"
)

// ListAllTools returns every fake HTTP tool ordered by slug.
func (s *CatalogStore) ListAllTools(ctx context.Context) ([]domain.HTTPTool, error) {
	defer s.lock(ctx)()
	items := make([]domain.HTTPTool, 0, len(s.tools))
	for _, tool := range s.tools {
		items = append(items, cloneHTTPTool(tool))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Slug < items[j].Slug })
	return items, nil
}

// ListAllAPIConnections returns every fake API connection ordered by slug.
func (s *CatalogStore) ListAllAPIConnections(ctx context.Context) ([]domain.APIConnection, error) {
	defer s.lock(ctx)()
	items := make([]domain.APIConnection, 0, len(s.connections))
	for _, connection := range s.connections {
		items = append(items, cloneAPIConnection(connection))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Slug < items[j].Slug })
	return items, nil
}
