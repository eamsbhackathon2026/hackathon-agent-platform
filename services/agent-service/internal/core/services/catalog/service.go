// Package catalog implements connection and assistant configuration workflows.
package catalog

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"context"
	"errors"
)

// Dependencies supplies storage, encryption and guarded provider access.
type Dependencies struct {
	Providers outbound.ProviderRepository
	Agents    outbound.AgentRepository
	Tx        outbound.TxManager
	Cipher    outbound.SecretCipher
	Factory   outbound.LLMClientFactory
	URLs      outbound.URLPolicy
	Clock     outbound.Clock
	IDs       outbound.IDGenerator
}

// Service manages the shared workspace catalog.
type Service struct{ Dependencies }

// NewService requires all ports so connection operations cannot bypass policy.
func NewService(d Dependencies) (*Service, error) {
	if d.Providers == nil || d.Agents == nil || d.Tx == nil || d.Cipher == nil || d.Factory == nil || d.URLs == nil || d.Clock == nil || d.IDs == nil {
		return nil, errors.New("catalog dependencies are required")
	}
	return &Service{d}, nil
}

func (s *Service) mutate(ctx context.Context, fn func(context.Context) error) error {
	return s.Tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.Providers.LockCatalog(ctx); err != nil {
			return err
		}
		return fn(ctx)
	})
}

func (s *Service) validateProvider(p domain.Provider) error {
	if err := domain.ValidateProvider(p); err != nil {
		return err
	}
	if p.BaseURL != nil && s.URLs.ValidateURL(*p.BaseURL) != nil {
		return domain.Invalid("base_url", "Địa chỉ kết nối không được phép. Kiểm tra địa chỉ hoặc cấu hình mạng.")
	}
	return nil
}

var _ inbound.ProviderUseCase = (*Service)(nil)
var _ inbound.AgentUseCase = (*Service)(nil)
