package catalog

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"context"
	"github.com/google/uuid"
)

// CreateProvider validates and encrypts the supplied key before persistence.
func (s *Service) CreateProvider(ctx context.Context, principal domain.Principal, c inbound.ProviderCreateCommand) (p domain.Provider, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	p = domain.Provider{Name: c.Name, Kind: c.Kind, BaseURL: c.BaseURL, DefaultModel: c.DefaultModel, Status: domain.ConnectionUnchecked, Revision: 1, CreatedAt: s.Clock.Now(), UpdatedAt: s.Clock.Now()}
	if err = s.validateProvider(p); err != nil {
		return
	}
	if err = domain.ValidateProviderKey(c.APIKey); err != nil {
		return
	}
	if p.ID, err = s.IDs.NewID(); err != nil {
		return
	}
	if err = s.setKey(&p, &c.APIKey); err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error { return s.Providers.CreateProvider(ctx, p) })
	return
}

// GetProvider returns encrypted state for safe DTO mapping at the boundary.
func (s *Service) GetProvider(ctx context.Context, principal domain.Principal, id uuid.UUID) (domain.Provider, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return domain.Provider{}, err
	}
	return s.Providers.GetProvider(ctx, id)
}

// ListProviders uses stable descending creation-time pagination.
func (s *Service) ListProviders(ctx context.Context, principal domain.Principal, req inbound.PageRequest) (page inbound.ProviderPage, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return
	}
	options, err := pageOptions(req)
	if err != nil {
		return
	}
	page.Items, err = s.Providers.ListProviders(ctx, options)
	if err != nil {
		return
	}
	if len(page.Items) == options.Limit {
		page.Items = page.Items[:len(page.Items)-1]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(domain.PageCursor{ID: last.ID, CreatedAt: last.CreatedAt})
	}
	return
}

// UpdateProvider merges changes under lock, preserving omitted credentials.
func (s *Service) UpdateProvider(ctx context.Context, principal domain.Principal, id uuid.UUID, c inbound.ProviderUpdateCommand) (p domain.Provider, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error {
		var e error
		p, e = s.Providers.GetProviderForUpdate(ctx, id)
		if e != nil {
			return e
		}
		if c.Name != nil {
			p.Name = *c.Name
		}
		if c.Kind != nil {
			p.Kind = *c.Kind
		}
		if c.BaseURL.Set {
			p.BaseURL = c.BaseURL.Value
		}
		if c.DefaultModel.Set {
			p.DefaultModel = c.DefaultModel.Value
		}
		if e = s.validateProvider(p); e != nil {
			return e
		}
		if c.APIKey.Set {
			if e = s.setKey(&p, c.APIKey.Value); e != nil {
				return e
			}
		}
		if c.Kind != nil || c.BaseURL.Set || c.DefaultModel.Set || c.APIKey.Set {
			p.Status = domain.ConnectionUnchecked
			p.LastError = nil
			p.LastCheckedAt = nil
		}
		p.UpdatedAt = s.Clock.Now()
		p, e = s.Providers.UpdateProvider(ctx, p)
		return e
	})
	return
}

// DeleteProvider prevents removal while active assistants reference the connection.
func (s *Service) DeleteProvider(ctx context.Context, principal domain.Principal, id uuid.UUID) error {
	if err := domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return err
	}
	return s.mutate(ctx, func(ctx context.Context) error {
		if _, err := s.Providers.GetProviderForUpdate(ctx, id); err != nil {
			return err
		}
		agents, err := s.Agents.ListAgentsByProvider(ctx, id)
		if err != nil {
			return err
		}
		if len(agents) > 0 {
			return &domain.Error{Kind: domain.ErrConflict, Detail: "Chuyển kết nối AI của các trợ lý liên quan trước khi xóa.", RelatedAgents: agents}
		}
		return s.Providers.DeleteProvider(ctx, id)
	})
}

func (s *Service) setKey(p *domain.Provider, key *string) error {
	if key == nil {
		p.APIKeyCiphertext = nil
		p.APIKeyHint = nil
		return nil
	}
	if err := domain.ValidateProviderKey(*key); err != nil {
		return err
	}
	encrypted, err := s.Cipher.Encrypt([]byte(*key), []byte("provider:"+p.ID.String()))
	if err != nil {
		return err
	}
	p.APIKeyCiphertext = encrypted
	runes := []rune(*key)
	hint := "****"
	if len(runes) > 4 {
		hint = string(runes[len(runes)-4:])
	}
	p.APIKeyHint = &hint
	return nil
}
