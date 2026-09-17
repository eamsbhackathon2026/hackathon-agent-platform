package tooling

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// CreateTool validates, encrypts and persists an HTTP tool.
func (s *Service) CreateTool(ctx context.Context, principal domain.Principal, command inbound.HTTPToolCreateCommand) (tool domain.HTTPTool, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	now := s.Clock.Now()
	tool = domain.HTTPTool{ConnectionID: cloneUUIDPointer(command.ConnectionID), Slug: command.Slug, DisplayName: command.DisplayName, Description: command.Description, Method: command.Method, URLTemplate: command.URLTemplate, Params: cloneParams(command.Params), PublicHeaders: cloneHeaders(command.PublicHeaders), TimeoutSeconds: command.TimeoutSeconds, CreatedAt: now, UpdatedAt: now}
	if err = s.validateHTTPTool(ctx, tool, command.SecretHeaders); err != nil {
		return
	}
	if tool.ID, err = s.IDs.NewID(); err != nil {
		return
	}
	tool.SecretHeadersCiphertext, tool.SecretHeaderNames, err = s.encryptHeaders("tool", tool.ID, command.SecretHeaders)
	if err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error { return s.Tools.CreateTool(ctx, tool) })
	return
}

// GetTool returns one saved HTTP tool without exposing secret values.
func (s *Service) GetTool(ctx context.Context, principal domain.Principal, id uuid.UUID) (domain.HTTPTool, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return domain.HTTPTool{}, err
	}
	return s.Tools.GetTool(ctx, id)
}

// ListTools returns a cursor-paginated collection of HTTP tools.
func (s *Service) ListTools(ctx context.Context, principal domain.Principal, request inbound.PageRequest) (page inbound.HTTPToolPage, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return
	}
	options, err := pageOptions(request)
	if err != nil {
		return page, err
	}
	page.Items, err = s.Tools.ListTools(ctx, options)
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

// UpdateTool validates and persists a partial HTTP tool update.
func (s *Service) UpdateTool(ctx context.Context, principal domain.Principal, id uuid.UUID, command inbound.HTTPToolUpdateCommand) (tool domain.HTTPTool, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error {
		tool, err = s.Tools.GetToolForUpdate(ctx, id)
		if err != nil {
			return err
		}
		secrets, decryptErr := s.decryptHeaders("tool", id, tool.SecretHeadersCiphertext)
		if decryptErr != nil {
			return decryptErr
		}
		if command.Slug != nil {
			tool.Slug = *command.Slug
		}
		if command.ConnectionID.Set {
			tool.ConnectionID = cloneUUIDPointer(command.ConnectionID.Value)
		}
		if command.DisplayName != nil {
			tool.DisplayName = *command.DisplayName
		}
		if command.Description != nil {
			tool.Description = *command.Description
		}
		if command.Method != nil {
			tool.Method = *command.Method
		}
		if command.URLTemplate != nil {
			tool.URLTemplate = *command.URLTemplate
		}
		if command.Params != nil {
			tool.Params = cloneParams(*command.Params)
		}
		if command.PublicHeaders != nil {
			tool.PublicHeaders = cloneHeaders(*command.PublicHeaders)
		}
		if command.TimeoutSeconds != nil {
			tool.TimeoutSeconds = *command.TimeoutSeconds
		}
		if command.SecretHeaders.Set {
			secrets = cloneHeadersValue(command.SecretHeaders.Value)
		}
		if err = s.validateHTTPTool(ctx, tool, secrets); err != nil {
			return err
		}
		if command.SecretHeaders.Set {
			tool.SecretHeadersCiphertext, tool.SecretHeaderNames, err = s.encryptHeaders("tool", id, secrets)
			if err != nil {
				return err
			}
		}
		tool.UpdatedAt = s.Clock.Now()
		return s.Tools.UpdateTool(ctx, tool)
	})
	return
}

// DeleteTool removes an HTTP tool and its agent bindings.
func (s *Service) DeleteTool(ctx context.Context, principal domain.Principal, id uuid.UUID) error {
	if err := domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return err
	}
	return s.mutate(ctx, func(ctx context.Context) error {
		if _, err := s.Tools.GetToolForUpdate(ctx, id); err != nil {
			return err
		}
		return s.Tools.DeleteTool(ctx, id)
	})
}

// TestTool executes one saved HTTP tool and returns a sanitized result.
func (s *Service) TestTool(ctx context.Context, principal domain.Principal, id uuid.UUID, args json.RawMessage) (result inbound.ToolTestResult, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	tool, err := s.Tools.GetTool(ctx, id)
	if err != nil {
		return result, err
	}
	connections := map[uuid.UUID]apiConnectionSnapshot{}
	if tool.ConnectionID != nil {
		connection, loadErr := s.Connections.GetAPIConnection(ctx, *tool.ConnectionID)
		if loadErr != nil {
			return result, loadErr
		}
		connectionSecrets, decryptErr := s.decryptHeaders("api_connection", connection.ID, connection.SecretHeadersCiphertext)
		if decryptErr != nil {
			return result, decryptErr
		}
		connections[connection.ID] = apiConnectionSnapshot{connection: connection, secrets: connectionSecrets}
	}
	tool, secrets, err := s.resolveHTTPTool(tool, connections)
	if err != nil {
		return result, err
	}
	started := time.Now()
	invocation, invokeErr := s.HTTP.Invoke(ctx, tool, secrets, args)
	result.Duration = time.Since(started)
	if invokeErr != nil {
		result.Failure = toolFailure(invokeErr)
		return result, nil
	}
	result.OK = !invocation.IsError
	result.StatusCode = invocation.StatusCode
	result.Body = invocation.Body
	result.Truncated = invocation.Truncated
	if invocation.IsError {
		result.Failure = &domain.ProviderFailure{Code: "tool_failed", Message: domain.ToolErrorGuidance(invocation.StatusCode)}
	}
	return result, nil
}

func (s *Service) validateHTTPTool(ctx context.Context, tool domain.HTTPTool, secrets map[string]string) error {
	if err := domain.ValidateHTTPTool(tool, secrets); err != nil {
		return err
	}
	resolvedURL := tool.URLTemplate
	if tool.ConnectionID != nil {
		connection, err := s.Connections.GetAPIConnection(ctx, *tool.ConnectionID)
		if err != nil {
			return err
		}
		resolvedURL, err = domain.JoinAPIConnectionURL(connection.BaseURL, tool.URLTemplate)
		if err != nil {
			return err
		}
	}
	if err := s.URLs.ValidateURL(resolvedURL); err != nil {
		return domain.Invalid("url_template", "Địa chỉ công cụ không được phép. Kiểm tra địa chỉ hoặc cấu hình mạng.")
	}
	return nil
}

func toolFailure(err error) *domain.ProviderFailure {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &domain.ProviderFailure{Code: "tool_failed", Message: "Công cụ hết thời gian phản hồi."}
	// Two different problems used to share one message, which sent people chasing the
	// network policy when the target service was simply down (or the other way round).
	case errors.Is(err, domain.ErrEgressDenied):
		return &domain.ProviderFailure{Code: "tool_failed", Message: "Địa chỉ công cụ bị chính sách mạng chặn. Kiểm tra lại địa chỉ hoặc cấu hình mạng."}
	case errors.Is(err, domain.ErrProviderUnreachable):
		return &domain.ProviderFailure{Code: "tool_failed", Message: "Không kết nối được tới hệ thống đích. Kiểm tra dịch vụ đích đang chạy và địa chỉ đúng."}
	case errors.Is(err, domain.ErrValidation), errors.Is(err, domain.ErrProviderBadRequest):
		return &domain.ProviderFailure{Code: "tool_failed", Message: "Đối số hoặc cấu hình công cụ không hợp lệ."}
	default:
		return &domain.ProviderFailure{Code: "tool_failed", Message: "Không thể chạy công cụ."}
	}
}

func cloneParams(value []domain.ToolParam) []domain.ToolParam {
	return append([]domain.ToolParam{}, value...)
}

func cloneHeaders(value map[string]string) map[string]string {
	result := make(map[string]string, len(value))
	for name, header := range value {
		result[name] = header
	}
	return result
}

func cloneHeadersValue(value *map[string]string) map[string]string {
	if value == nil {
		return map[string]string{}
	}
	return cloneHeaders(*value)
}

func cloneUUIDPointer(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

var _ inbound.ToolUseCase = (*Service)(nil)
