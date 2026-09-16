package fakes

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"context"
	"sync"
)

// LLMStep is one scripted generation, including ordered deltas and final metadata.
type LLMStep struct {
	Deltas []domain.LLMDelta
	Result domain.LLMResult
	Err    error
}

// ScriptedLLM is reusable for multi-turn run tests. Configure fields before calls.
type ScriptedLLM struct {
	mu          sync.Mutex
	Models      []string
	ModelsError error
	Steps       []LLMStep
	ModelFunc   func(context.Context) ([]string, error)
	StreamFunc  func(context.Context, domain.LLMRequest, func(domain.LLMDelta)) (domain.LLMResult, error)
	requests    []domain.LLMRequest
	modelCalls  int
}

// Stream consumes one step and emits its ordered text fragments.
func (s *ScriptedLLM) Stream(ctx context.Context, req domain.LLMRequest, emit func(domain.LLMDelta)) (domain.LLMResult, error) {
	s.mu.Lock()
	s.requests = append(s.requests, req)
	fn := s.StreamFunc
	step := LLMStep{}
	if len(s.Steps) > 0 {
		step = s.Steps[0]
		s.Steps = s.Steps[1:]
	}
	s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.LLMResult{}, err
	}
	if fn != nil {
		return fn(ctx, req, emit)
	}
	for _, d := range step.Deltas {
		if err := ctx.Err(); err != nil {
			return domain.LLMResult{}, err
		}
		if emit != nil {
			emit(d)
		}
	}
	return step.Result, step.Err
}

// ListModels records each list attempt and returns copied IDs.
func (s *ScriptedLLM) ListModels(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	s.modelCalls++
	fn := s.ModelFunc
	models := append([]string(nil), s.Models...)
	err := s.ModelsError
	s.mu.Unlock()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if fn != nil {
		return fn(ctx)
	}
	return models, err
}

// Requests returns the submitted requests for assertions after execution.
func (s *ScriptedLLM) Requests() []domain.LLMRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.LLMRequest(nil), s.requests...)
}

// ModelCalls reports how many listing attempts occurred.
func (s *ScriptedLLM) ModelCalls() int { s.mu.Lock(); defer s.mu.Unlock(); return s.modelCalls }

// FakeFactory returns a configured client and records decrypted connections in tests only.
type FakeFactory struct {
	mu          sync.Mutex
	Client      outbound.LLMClient
	Err         error
	connections []domain.ProviderConnection
}

// New records connection parameters before returning its scripted outcome.
func (f *FakeFactory) New(_ context.Context, c domain.ProviderConnection) (outbound.LLMClient, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connections = append(f.connections, c)
	return f.Client, f.Err
}

// Connections returns a copied call history; never use this fake in production.
func (f *FakeFactory) Connections() []domain.ProviderConnection {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.ProviderConnection(nil), f.connections...)
}

var _ outbound.LLMClient = (*ScriptedLLM)(nil)
var _ outbound.LLMClientFactory = (*FakeFactory)(nil)
