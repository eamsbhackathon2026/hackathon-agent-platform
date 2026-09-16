// Package skills manages uploaded skill instructions and assistant bindings.
package skills

import (
	"context"
	"errors"

	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// Dependencies supplies persistence and identity boundaries for Skill Hub.
type Dependencies struct {
	Skills outbound.SkillRepository
	Agents outbound.AgentRepository
	Tx     outbound.TxManager
	Clock  outbound.Clock
	IDs    outbound.IDGenerator
}

// Service manages immutable skill packages and resolves run-scoped instructions.
type Service struct{ Dependencies }

// NewService fails closed when a required boundary is missing.
func NewService(dependencies Dependencies) (*Service, error) {
	if dependencies.Skills == nil || dependencies.Agents == nil || dependencies.Tx == nil || dependencies.Clock == nil || dependencies.IDs == nil {
		return nil, errors.New("skill dependencies are required")
	}
	return &Service{Dependencies: dependencies}, nil
}

func (s *Service) mutate(ctx context.Context, operation func(context.Context) error) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.Skills.LockSkills(ctx); err != nil {
			return err
		}
		return operation(ctx)
	})
}
