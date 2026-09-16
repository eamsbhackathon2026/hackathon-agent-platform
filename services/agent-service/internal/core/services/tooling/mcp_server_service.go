package tooling

import (
	"context"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// CreateMCPServer validates, encrypts and persists an MCP endpoint.
func (s *Service) CreateMCPServer(ctx context.Context, principal domain.Principal, command inbound.MCPServerCreateCommand) (server domain.MCPServer, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	now := s.Clock.Now()
	server = domain.MCPServer{Slug: command.Slug, DisplayName: command.DisplayName, URL: command.URL, AllowedTools: cloneStringsPointer(command.AllowedTools), Tools: []domain.MCPTool{}, Status: domain.ConnectionUnchecked, CreatedAt: now, UpdatedAt: now, Revision: 1}
	if err = s.validateMCPServer(server, command.SecretHeaders); err != nil {
		return
	}
	if server.ID, err = s.IDs.NewID(); err != nil {
		return
	}
	server.SecretHeadersCiphertext, server.SecretHeaderNames, err = s.encryptHeaders("mcp", server.ID, command.SecretHeaders)
	if err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error { return s.MCPServers.CreateMCPServer(ctx, server) })
	return
}

// GetMCPServer returns one endpoint and its cached tool declarations.
func (s *Service) GetMCPServer(ctx context.Context, principal domain.Principal, id uuid.UUID) (domain.MCPServer, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return domain.MCPServer{}, err
	}
	return s.MCPServers.GetMCPServer(ctx, id)
}

// ListMCPServers returns a cursor-paginated collection of MCP endpoints.
func (s *Service) ListMCPServers(ctx context.Context, principal domain.Principal, request inbound.PageRequest) (page inbound.MCPServerPage, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return
	}
	options, err := pageOptions(request)
	if err != nil {
		return page, err
	}
	page.Items, err = s.MCPServers.ListMCPServers(ctx, options)
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

// UpdateMCPServer validates and persists a partial endpoint update.
func (s *Service) UpdateMCPServer(ctx context.Context, principal domain.Principal, id uuid.UUID, command inbound.MCPServerUpdateCommand) (server domain.MCPServer, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error {
		server, err = s.MCPServers.GetMCPServerForUpdate(ctx, id)
		if err != nil {
			return err
		}
		secrets, decryptErr := s.decryptHeaders("mcp", id, server.SecretHeadersCiphertext)
		if decryptErr != nil {
			return decryptErr
		}
		endpointChanged := command.URL != nil || command.SecretHeaders.Set
		if command.Slug != nil {
			server.Slug = *command.Slug
		}
		if command.DisplayName != nil {
			server.DisplayName = *command.DisplayName
		}
		if command.URL != nil {
			server.URL = *command.URL
		}
		if command.AllowedTools.Set {
			server.AllowedTools = cloneStringsPointer(command.AllowedTools.Value)
		}
		if command.SecretHeaders.Set {
			secrets = cloneHeadersValue(command.SecretHeaders.Value)
		}
		if err = s.validateMCPServer(server, secrets); err != nil {
			return err
		}
		if command.SecretHeaders.Set {
			server.SecretHeadersCiphertext, server.SecretHeaderNames, err = s.encryptHeaders("mcp", id, secrets)
			if err != nil {
				return err
			}
		}
		if endpointChanged {
			server.Tools = []domain.MCPTool{}
			server.Status = domain.ConnectionUnchecked
			server.LastError = nil
			server.LastSyncedAt = nil
		}
		server.UpdatedAt = s.Clock.Now()
		server, err = s.MCPServers.UpdateMCPServer(ctx, server)
		return err
	})
	return
}

// DeleteMCPServer removes an endpoint and its agent bindings.
func (s *Service) DeleteMCPServer(ctx context.Context, principal domain.Principal, id uuid.UUID) error {
	if err := domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return err
	}
	return s.mutate(ctx, func(ctx context.Context) error {
		if _, err := s.MCPServers.GetMCPServerForUpdate(ctx, id); err != nil {
			return err
		}
		return s.MCPServers.DeleteMCPServer(ctx, id)
	})
}

// RefreshMCPServer fetches and conditionally stores the endpoint's tool catalog.
func (s *Service) RefreshMCPServer(ctx context.Context, principal domain.Principal, id uuid.UUID) (domain.MCPServer, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return domain.MCPServer{}, err
	}
	server, err := s.MCPServers.GetMCPServer(ctx, id)
	if err != nil {
		return domain.MCPServer{}, err
	}
	secrets, err := s.decryptHeaders("mcp", id, server.SecretHeadersCiphertext)
	if err != nil {
		return domain.MCPServer{}, err
	}
	tools := server.Tools
	status := domain.ConnectionOK
	var failure *domain.ProviderFailure
	session, connectErr := s.MCP.Connect(ctx, server, secrets)
	if connectErr == nil {
		listCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		tools, connectErr = session.ListTools(listCtx)
		cancel()
		closeErr := session.Close()
		if connectErr == nil {
			connectErr = closeErr
		}
	}
	if connectErr != nil {
		status = domain.ConnectionFailing
		failure = toolFailure(connectErr)
	}
	at := s.Clock.Now()
	applied, err := s.MCPServers.RecordMCPServerSync(ctx, id, server.Revision, tools, status, failure, at)
	if err != nil {
		return domain.MCPServer{}, err
	}
	if !applied {
		return domain.MCPServer{}, &domain.Error{Kind: domain.ErrConflict, Detail: "Cấu hình máy chủ đã thay đổi; hãy làm mới lại."}
	}
	return s.MCPServers.GetMCPServer(ctx, id)
}

func (s *Service) validateMCPServer(server domain.MCPServer, secrets map[string]string) error {
	if err := domain.ValidateMCPServer(server, secrets); err != nil {
		return err
	}
	if err := s.URLs.ValidateURL(server.URL); err != nil {
		return domain.Invalid("url", "Địa chỉ máy chủ công cụ không được phép. Kiểm tra địa chỉ hoặc cấu hình mạng.")
	}
	return nil
}

func cloneStringsPointer(value *[]string) *[]string {
	if value == nil {
		return nil
	}
	result := append([]string{}, (*value)...)
	return &result
}

var _ inbound.MCPServerUseCase = (*Service)(nil)
