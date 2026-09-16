// Package tooling manages saved HTTP/MCP tools and resolves run-scoped tool sets.
package tooling

import (
	"context"
	"errors"

	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// Dependencies supplies persistence, encryption and guarded outbound adapters.
type Dependencies struct {
	Tools       outbound.ToolRepository
	Connections outbound.APIConnectionRepository
	MCPServers  outbound.MCPServerRepository
	Bindings    outbound.AgentToolBindingRepository
	Agents      outbound.AgentRepository
	Tx          outbound.TxManager
	Cipher      outbound.SecretCipher
	HTTP        outbound.HTTPToolInvoker
	MCP         outbound.MCPConnector
	URLs        outbound.URLPolicy
	Clock       outbound.Clock
	IDs         outbound.IDGenerator
}

// Service implements configuration use cases and the run-engine resolver.
type Service struct{ Dependencies }

// NewService validates dependencies and creates the tooling application service.
func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.Tools == nil || dependencies.Connections == nil || dependencies.MCPServers == nil || dependencies.Bindings == nil || dependencies.Agents == nil || dependencies.Tx == nil || dependencies.Cipher == nil || dependencies.HTTP == nil || dependencies.MCP == nil || dependencies.URLs == nil || dependencies.Clock == nil || dependencies.IDs == nil {
		return nil, errors.New("tooling dependencies are required")
	}
	return &Service{Dependencies: dependencies}, nil
}

func (s *Service) mutate(ctx context.Context, operation func(context.Context) error) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.Tools.LockTooling(ctx); err != nil {
			return err
		}
		return operation(ctx)
	})
}
