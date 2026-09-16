package fakes

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// ToolSet records calls and returns scripted results in order.
type ToolSet struct {
	mu      sync.Mutex
	Tools   []domain.ToolSpec
	Results []domain.ToolResult
	Calls   []domain.ToolCall
	Run     func(context.Context, domain.ToolCall) domain.ToolResult
}

// Specs returns copied scripted tool declarations.
func (s *ToolSet) Specs() []domain.ToolSpec { return append([]domain.ToolSpec(nil), s.Tools...) }

// Execute records a call and returns its scripted result.
func (s *ToolSet) Execute(ctx context.Context, call domain.ToolCall) domain.ToolResult {
	s.mu.Lock()
	s.Calls = append(s.Calls, call)
	fn := s.Run
	var result domain.ToolResult
	if len(s.Results) > 0 {
		result, s.Results = s.Results[0], s.Results[1:]
	}
	s.mu.Unlock()
	if fn != nil {
		return fn(ctx, call)
	}
	return result
}

// Close releases no resources in the fake.
func (s *ToolSet) Close() error { return nil }

// ToolResolver returns one configured fake set.
type ToolResolver struct {
	Set outbound.ToolSet
	Err error
}

// Resolve returns the configured result.
func (r ToolResolver) Resolve(context.Context, uuid.UUID) (outbound.ToolSet, error) {
	return r.Set, r.Err
}

var _ outbound.ToolSet = (*ToolSet)(nil)
var _ outbound.ToolResolver = ToolResolver{}
