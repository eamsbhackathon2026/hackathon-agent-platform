package tooling

import (
	"context"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// CreateAPIConnection validates, encrypts and persists reusable API configuration.
func (s *Service) CreateAPIConnection(ctx context.Context, principal domain.Principal, command inbound.APIConnectionCreateCommand) (connection domain.APIConnection, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	now := s.Clock.Now()
	connection = domain.APIConnection{Slug: command.Slug, DisplayName: command.DisplayName, BaseURL: command.BaseURL, PublicHeaders: cloneHeaders(command.PublicHeaders), CreatedAt: now, UpdatedAt: now}
	if err = s.validateAPIConnection(connection, command.SecretHeaders); err != nil {
		return
	}
	if connection.ID, err = s.IDs.NewID(); err != nil {
		return
	}
	connection.SecretHeadersCiphertext, connection.SecretHeaderNames, err = s.encryptHeaders("api_connection", connection.ID, command.SecretHeaders)
	if err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error { return s.Connections.CreateAPIConnection(ctx, connection) })
	return
}

// GetAPIConnection returns reusable API configuration without secret values.
func (s *Service) GetAPIConnection(ctx context.Context, principal domain.Principal, id uuid.UUID) (domain.APIConnection, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return domain.APIConnection{}, err
	}
	return s.Connections.GetAPIConnection(ctx, id)
}

// ListAPIConnections returns cursor-paginated reusable API configuration.
func (s *Service) ListAPIConnections(ctx context.Context, principal domain.Principal, request inbound.PageRequest) (page inbound.APIConnectionPage, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return
	}
	options, err := pageOptions(request)
	if err != nil {
		return page, err
	}
	page.Items, err = s.Connections.ListAPIConnections(ctx, options)
	if err != nil {
		return page, err
	}
	if len(page.Items) == options.Limit {
		page.Items = page.Items[:len(page.Items)-1]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeCursor(domain.PageCursor{ID: last.ID, CreatedAt: last.CreatedAt})
	}
	return page, nil
}

// UpdateAPIConnection applies a partial update while preserving omitted secrets.
func (s *Service) UpdateAPIConnection(ctx context.Context, principal domain.Principal, id uuid.UUID, command inbound.APIConnectionUpdateCommand) (connection domain.APIConnection, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error {
		connection, err = s.Connections.GetAPIConnectionForUpdate(ctx, id)
		if err != nil {
			return err
		}
		secrets, decryptErr := s.decryptHeaders("api_connection", id, connection.SecretHeadersCiphertext)
		if decryptErr != nil {
			return decryptErr
		}
		if command.Slug != nil {
			connection.Slug = *command.Slug
		}
		if command.DisplayName != nil {
			connection.DisplayName = *command.DisplayName
		}
		if command.BaseURL != nil {
			connection.BaseURL = *command.BaseURL
		}
		if command.PublicHeaders != nil {
			connection.PublicHeaders = cloneHeaders(*command.PublicHeaders)
		}
		if command.SecretHeaders.Set {
			secrets = cloneHeadersValue(command.SecretHeaders.Value)
		}
		if err = s.validateAPIConnection(connection, secrets); err != nil {
			return err
		}
		if command.SecretHeaders.Set {
			connection.SecretHeadersCiphertext, connection.SecretHeaderNames, err = s.encryptHeaders("api_connection", id, secrets)
			if err != nil {
				return err
			}
		}
		connection.UpdatedAt = s.Clock.Now()
		return s.Connections.UpdateAPIConnection(ctx, connection)
	})
	return
}

// DeleteAPIConnection removes an unused connection and protects referenced tools.
func (s *Service) DeleteAPIConnection(ctx context.Context, principal domain.Principal, id uuid.UUID) error {
	if err := domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return err
	}
	return s.mutate(ctx, func(ctx context.Context) error {
		if _, err := s.Connections.GetAPIConnectionForUpdate(ctx, id); err != nil {
			return err
		}
		count, err := s.Connections.CountToolsByAPIConnection(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return domain.ErrConflict
		}
		return s.Connections.DeleteAPIConnection(ctx, id)
	})
}

func (s *Service) validateAPIConnection(connection domain.APIConnection, secrets map[string]string) error {
	if err := domain.ValidateAPIConnection(connection, secrets); err != nil {
		return err
	}
	if err := s.URLs.ValidateURL(connection.BaseURL); err != nil {
		return domain.Invalid("base_url", "Địa chỉ kết nối không được phép. Kiểm tra địa chỉ hoặc cấu hình mạng.")
	}
	return nil
}

var _ inbound.APIConnectionUseCase = (*Service)(nil)
