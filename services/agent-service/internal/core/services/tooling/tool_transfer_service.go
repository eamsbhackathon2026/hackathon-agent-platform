package tooling

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// ExportTools returns every HTTP tool with the connections they use, without secrets.
func (s *Service) ExportTools(ctx context.Context, principal domain.Principal) (bundle domain.ToolBundle, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesRead, nil); err != nil {
		return
	}
	tools, err := s.Tools.ListAllTools(ctx)
	if err != nil {
		return bundle, err
	}
	connections, err := s.Connections.ListAllAPIConnections(ctx)
	if err != nil {
		return bundle, err
	}
	byID := make(map[uuid.UUID]domain.APIConnection, len(connections))
	for _, connection := range connections {
		byID[connection.ID] = connection
	}
	if len(tools) > domain.ToolBundleMaxTools {
		return bundle, domain.Invalid("tools", fmt.Sprintf("Một file xuất có tối đa %d công cụ; hệ thống đang có %d nên file sẽ không nhập lại được.", domain.ToolBundleMaxTools, len(tools)))
	}
	bundle = domain.ToolBundle{Format: domain.ToolBundleFormat, Version: domain.ToolBundleVersion, ExportedAt: s.Clock.Now(), Connections: []domain.ToolBundleConnection{}, Tools: make([]domain.ToolBundleTool, 0, len(tools))}
	used := map[uuid.UUID]bool{}
	for _, tool := range tools {
		entry := domain.ToolBundleTool{Slug: tool.Slug, DisplayName: tool.DisplayName, Description: tool.Description, Method: tool.Method, URLTemplate: tool.URLTemplate, Params: cloneParams(tool.Params), PublicHeaders: cloneHeaders(tool.PublicHeaders), TimeoutSeconds: tool.TimeoutSeconds, SecretHeaderNames: append([]string{}, tool.SecretHeaderNames...)}
		if tool.ConnectionID != nil {
			connection, ok := byID[*tool.ConnectionID]
			if !ok {
				return domain.ToolBundle{}, fmt.Errorf("tool %s references a missing connection", tool.ID)
			}
			slug := connection.Slug
			entry.ConnectionSlug = &slug
			if !used[connection.ID] {
				used[connection.ID] = true
				bundle.Connections = append(bundle.Connections, domain.ToolBundleConnection{Slug: connection.Slug, DisplayName: connection.DisplayName, BaseURL: connection.BaseURL, PublicHeaders: cloneHeaders(connection.PublicHeaders), SecretHeaderNames: append([]string{}, connection.SecretHeaderNames...)})
			}
		}
		bundle.Tools = append(bundle.Tools, entry)
	}
	sort.Slice(bundle.Connections, func(i, j int) bool { return bundle.Connections[i].Slug < bundle.Connections[j].Slug })
	if len(bundle.Connections) > domain.ToolBundleMaxConnections {
		return domain.ToolBundle{}, domain.Invalid("connections", fmt.Sprintf("Một file xuất có tối đa %d kết nối API.", domain.ToolBundleMaxConnections))
	}
	return bundle, nil
}

// PreviewToolImport classifies bundle entries without writing anything.
func (s *Service) PreviewToolImport(ctx context.Context, principal domain.Principal, bundle domain.ToolBundle) ([]domain.ToolImportItem, error) {
	if err := domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return nil, err
	}
	plan, err := s.planImport(ctx, bundle, nil)
	if err != nil {
		return nil, err
	}
	return plan.items(), nil
}

// ImportTools writes a bundle in one transaction. The plan is rebuilt under the
// tooling lock so a change made after the preview cannot slip past the checks.
func (s *Service) ImportTools(ctx context.Context, principal domain.Principal, bundle domain.ToolBundle, decisions map[domain.ToolImportKey]domain.ToolImportAction) (result domain.ToolImportResult, err error) {
	if err = domain.Authorize(principal, domain.ActionResourcesWrite, nil); err != nil {
		return
	}
	err = s.mutate(ctx, func(ctx context.Context) error {
		result = domain.ToolImportResult{NeedsSecrets: []domain.ToolImportSecretReminder{}}
		plan, planErr := s.planImport(ctx, bundle, decisions)
		if planErr != nil {
			return planErr
		}
		if problems := importProblems(plan, decisions); len(problems) > 0 {
			return &domain.Error{Kind: domain.ErrValidation, Detail: "Một số mục chưa nhập được. Bỏ qua các mục này hoặc sửa file rồi thử lại.", Fields: problems}
		}
		// Tools may point at a saved connection that is not in the bundle at all.
		connectionIDs := make(map[string]uuid.UUID, len(plan.savedConnectionIDs))
		for slug, id := range plan.savedConnectionIDs {
			connectionIDs[slug] = id
		}
		for _, planned := range plan.connections {
			if planned.duplicate || decisions[planned.item.ToolImportKey] == domain.ToolImportSkip {
				result.Skipped++
				continue
			}
			id, reminder, writeErr := s.writeImportedConnection(ctx, planned)
			if writeErr != nil {
				return writeErr
			}
			connectionIDs[planned.entry.Slug] = id
			countImported(&result, planned.existing != nil, reminder)
		}
		for _, planned := range plan.tools {
			if planned.duplicate || decisions[planned.item.ToolImportKey] == domain.ToolImportSkip {
				result.Skipped++
				continue
			}
			reminder, writeErr := s.writeImportedTool(ctx, planned, connectionIDs)
			if writeErr != nil {
				return writeErr
			}
			countImported(&result, planned.existing != nil, reminder)
		}
		return nil
	})
	return
}

// importProblems lists every entry that would be written but cannot be: invalid
// entries that were not skipped, and existing slugs with no skip/overwrite choice.
func importProblems(plan importPlan, decisions map[domain.ToolImportKey]domain.ToolImportAction) []domain.FieldError {
	var problems []domain.FieldError
	check := func(item domain.ToolImportItem, duplicate bool) {
		action := decisions[item.ToolImportKey]
		if duplicate || action == domain.ToolImportSkip {
			return
		}
		prefix := fmt.Sprintf("%ss.%s", item.Kind, item.Slug)
		switch item.Status {
		case domain.ToolImportInvalid:
			for _, field := range item.Fields {
				name := prefix
				if field.Field != "" {
					name += "." + field.Field
				}
				problems = append(problems, domain.FieldError{Field: name, Message: field.Message})
			}
		case domain.ToolImportConflict:
			if action != domain.ToolImportOverwrite {
				problems = append(problems, domain.FieldError{Field: prefix, Message: "Mã này đã có trong hệ thống; chọn bỏ qua hoặc thay thế."})
			}
		}
	}
	for _, planned := range plan.connections {
		check(planned.item, planned.duplicate)
	}
	for _, planned := range plan.tools {
		check(planned.item, planned.duplicate)
	}
	return problems
}

var _ inbound.ToolTransferUseCase = (*Service)(nil)
