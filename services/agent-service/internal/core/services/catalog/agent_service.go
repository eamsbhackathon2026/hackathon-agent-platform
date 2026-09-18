package catalog

import (
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"context"
	"errors"
	"github.com/google/uuid"
)

// CreateAgent applies defaults and checks its connection under the catalog lock.
func (s *Service) CreateAgent(ctx context.Context, p domain.Principal, c inbound.AgentCreateCommand) (view domain.AgentView, err error) {
	if err = domain.Authorize(p, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	a := domain.Agent{Name: c.Name, Description: c.Description, ProviderID: c.ProviderID, Model: c.Model, SystemPrompt: c.SystemPrompt, Temperature: c.Temperature, MaxOutputTokens: c.MaxOutputTokens, ShowThinking: c.ShowThinking != nil && *c.ShowThinking, ContextWindowTokens: domain.DefaultContextWindowTokens, MaxIterations: 8, TimeoutSeconds: 120, CreatedBy: p.UserID, CreatedAt: s.Clock.Now(), UpdatedAt: s.Clock.Now()}
	if c.ContextWindowTokens != nil {
		a.ContextWindowTokens = *c.ContextWindowTokens
	}
	if c.MaxIterations != nil {
		a.MaxIterations = *c.MaxIterations
	}
	if c.TimeoutSeconds != nil {
		a.TimeoutSeconds = *c.TimeoutSeconds
	}
	if err = domain.ValidateAgent(a); err != nil {
		return
	}
	if a.ID, err = s.IDs.NewID(); err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error {
		provider, e := s.Providers.GetProvider(ctx, a.ProviderID)
		if e != nil {
			return e
		}
		if e = s.Agents.CreateAgent(ctx, a); e != nil {
			return e
		}
		view = agentView(a, provider)
		return nil
	})
	return
}

// UpdateAgent validates the complete merged configuration, including cleared optional values.
func (s *Service) UpdateAgent(ctx context.Context, p domain.Principal, id uuid.UUID, c inbound.AgentUpdateCommand) (view domain.AgentView, err error) {
	if err = domain.Authorize(p, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error {
		a, e := s.Agents.GetAgentForUpdate(ctx, id)
		if e != nil {
			return e
		}
		if c.Name != nil {
			a.Name = *c.Name
		}
		if c.Description != nil {
			a.Description = *c.Description
		}
		if c.ProviderID != nil {
			a.ProviderID = *c.ProviderID
		}
		if c.Model != nil {
			a.Model = *c.Model
		}
		if c.SystemPrompt != nil {
			a.SystemPrompt = *c.SystemPrompt
		}
		if c.Temperature.Set {
			a.Temperature = c.Temperature.Value
		}
		if c.ShowThinking != nil {
			a.ShowThinking = *c.ShowThinking
		}
		if c.MaxOutputTokens.Set {
			a.MaxOutputTokens = c.MaxOutputTokens.Value
		}
		if c.ContextWindowTokens != nil {
			a.ContextWindowTokens = *c.ContextWindowTokens
		}
		if c.MaxIterations != nil {
			a.MaxIterations = *c.MaxIterations
		}
		if c.TimeoutSeconds != nil {
			a.TimeoutSeconds = *c.TimeoutSeconds
		}
		if e = domain.ValidateAgent(a); e != nil {
			return e
		}
		provider, e := s.Providers.GetProvider(ctx, a.ProviderID)
		if e != nil {
			return e
		}
		a.UpdatedAt = s.Clock.Now()
		if e = s.Agents.UpdateAgent(ctx, a); e != nil {
			return e
		}
		view = agentView(a, provider)
		return nil
	})
	return
}

// DeleteAgent archives the assistant to preserve existing conversation history.
func (s *Service) DeleteAgent(ctx context.Context, p domain.Principal, id uuid.UUID) error {
	if err := domain.Authorize(p, domain.ActionResourcesWrite, nil); err != nil {
		return err
	}
	return s.mutate(ctx, func(ctx context.Context) error {
		if _, err := s.Agents.GetAgentForUpdate(ctx, id); err != nil {
			return err
		}
		return s.Agents.ArchiveAgent(ctx, id, s.Clock.Now())
	})
}

// GetAgent returns an active assistant and locally computed readiness.
func (s *Service) GetAgent(ctx context.Context, p domain.Principal, id uuid.UUID) (domain.AgentView, error) {
	if err := domain.Authorize(p, domain.ActionResourcesRead, nil); err != nil {
		return domain.AgentView{}, err
	}
	a, err := s.Agents.GetAgent(ctx, id)
	if err != nil {
		return domain.AgentView{}, err
	}
	provider, err := s.Providers.GetProvider(ctx, a.ProviderID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.AgentView{}, err
	}
	return agentView(a, provider), nil
}

// ListAgents reads each distinct connection once when computing readiness.
func (s *Service) ListAgents(ctx context.Context, p domain.Principal, req inbound.PageRequest) (page inbound.AgentPage, err error) {
	if err = domain.Authorize(p, domain.ActionResourcesRead, nil); err != nil {
		return
	}
	options, err := pageOptions(req)
	if err != nil {
		return
	}
	agents, err := s.Agents.ListAgents(ctx, options)
	if err != nil {
		return
	}
	if len(agents) == options.Limit {
		agents = agents[:len(agents)-1]
		last := agents[len(agents)-1]
		page.NextCursor = encodeCursor(domain.PageCursor{ID: last.ID, CreatedAt: last.CreatedAt})
	}
	providers := map[uuid.UUID]domain.Provider{}
	page.Items = []domain.AgentView{}
	for _, a := range agents {
		provider, ok := providers[a.ProviderID]
		if !ok {
			provider, err = s.Providers.GetProvider(ctx, a.ProviderID)
			if err != nil && !errors.Is(err, domain.ErrNotFound) {
				return page, err
			}
			providers[a.ProviderID] = provider
		}
		page.Items = append(page.Items, agentView(a, provider))
	}
	return page, nil
}

func agentView(a domain.Agent, p domain.Provider) domain.AgentView {
	view := domain.AgentView{Agent: a, Ready: true}
	if p.ID == uuid.Nil || len(p.APIKeyCiphertext) == 0 {
		view.ReadinessError = providerFailure(domain.ErrProviderNotConfigured)
	} else if p.Status == domain.ConnectionFailing {
		view.ReadinessError = p.LastError
		if view.ReadinessError == nil {
			view.ReadinessError = providerFailure(domain.ErrProviderUnreachable)
		}
	}
	view.Ready = view.ReadinessError == nil
	return view
}
