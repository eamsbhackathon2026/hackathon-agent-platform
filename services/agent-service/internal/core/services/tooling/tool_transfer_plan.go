package tooling

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// A duplicate is a repeated slug inside the bundle. Decisions are keyed by slug, so a
// duplicate cannot be decided on separately; the import always ignores it.
type connectionImport struct {
	item      domain.ToolImportItem
	entry     domain.ToolBundleConnection
	existing  *domain.APIConnection
	duplicate bool
}

type toolImport struct {
	item      domain.ToolImportItem
	entry     domain.ToolBundleTool
	existing  *domain.HTTPTool
	duplicate bool
}

type importPlan struct {
	connections        []connectionImport
	tools              []toolImport
	savedConnectionIDs map[string]uuid.UUID
}

func (p importPlan) items() []domain.ToolImportItem {
	items := make([]domain.ToolImportItem, 0, len(p.connections)+len(p.tools))
	for _, connection := range p.connections {
		items = append(items, connection.item)
	}
	for _, tool := range p.tools {
		items = append(items, tool.item)
	}
	return items
}

// planImport classifies every bundle entry against the saved configuration. Decisions
// matter because a tool is checked against whichever connection will actually exist:
// the bundle's version unless that entry is skipped, otherwise the saved one.
func (s *Service) planImport(ctx context.Context, bundle domain.ToolBundle, decisions map[domain.ToolImportKey]domain.ToolImportAction) (importPlan, error) {
	if err := validateBundleShape(bundle); err != nil {
		return importPlan{}, err
	}
	savedConnections, err := s.Connections.ListAllAPIConnections(ctx)
	if err != nil {
		return importPlan{}, err
	}
	savedTools, err := s.Tools.ListAllTools(ctx)
	if err != nil {
		return importPlan{}, err
	}
	connectionsBySlug := make(map[string]domain.APIConnection, len(savedConnections))
	for _, connection := range savedConnections {
		connectionsBySlug[connection.Slug] = connection
	}
	toolsBySlug := make(map[string]domain.HTTPTool, len(savedTools))
	for _, tool := range savedTools {
		toolsBySlug[tool.Slug] = tool
	}

	plan := importPlan{savedConnectionIDs: make(map[string]uuid.UUID, len(savedConnections))}
	for _, connection := range savedConnections {
		plan.savedConnectionIDs[connection.Slug] = connection.ID
	}
	// Tools resolve their connection through this map; nil marks a bundle entry that
	// cannot be used and has no saved fallback, so tools pointing at it are reported.
	usable := map[string]*domain.APIConnection{}
	seen := map[string]bool{}
	for _, entry := range bundle.Connections {
		key := domain.ToolImportKey{Kind: domain.ToolImportConnection, Slug: entry.Slug}
		planned := connectionImport{item: domain.ToolImportItem{ToolImportKey: key, DisplayName: entry.DisplayName}, entry: entry}
		if saved, ok := connectionsBySlug[entry.Slug]; ok {
			planned.existing = &saved
		}
		if seen[entry.Slug] {
			planned.duplicate, planned.item.Fields = true, duplicateSlugField()
			planned.item.Status = domain.ToolImportInvalid
			plan.connections = append(plan.connections, planned)
			continue
		}
		seen[entry.Slug] = true
		candidate := domain.APIConnection{Slug: entry.Slug, DisplayName: entry.DisplayName, BaseURL: entry.BaseURL, PublicHeaders: cloneHeaders(entry.PublicHeaders)}
		planned.item.Fields = fieldErrors(s.validateAPIConnection(candidate, placeholderSecrets(entry.SecretHeaderNames)))
		planned.item.Status = importStatus(planned.item.Fields, planned.existing != nil)
		// A skipped or unusable bundle entry leaves the saved connection (if any) in place.
		if decisions[key] == domain.ToolImportSkip || planned.item.Status == domain.ToolImportInvalid {
			usable[entry.Slug] = planned.existing
		} else {
			usable[entry.Slug] = &candidate
		}
		plan.connections = append(plan.connections, planned)
	}

	seen = map[string]bool{}
	for _, entry := range bundle.Tools {
		key := domain.ToolImportKey{Kind: domain.ToolImportTool, Slug: entry.Slug}
		planned := toolImport{item: domain.ToolImportItem{ToolImportKey: key, DisplayName: entry.DisplayName}, entry: entry}
		if saved, ok := toolsBySlug[entry.Slug]; ok {
			planned.existing = &saved
		}
		if seen[entry.Slug] {
			planned.duplicate, planned.item.Fields = true, duplicateSlugField()
			planned.item.Status = domain.ToolImportInvalid
			plan.tools = append(plan.tools, planned)
			continue
		}
		seen[entry.Slug] = true
		planned.item.Fields = s.validateBundleTool(entry, usable, connectionsBySlug)
		planned.item.Status = importStatus(planned.item.Fields, planned.existing != nil)
		plan.tools = append(plan.tools, planned)
	}
	return plan, nil
}

func (s *Service) validateBundleTool(entry domain.ToolBundleTool, usable map[string]*domain.APIConnection, saved map[string]domain.APIConnection) []domain.FieldError {
	candidate := bundleToolModel(entry, nil)
	var connection *domain.APIConnection
	if entry.ConnectionSlug != nil {
		slug := *entry.ConnectionSlug
		resolved, inBundle := usable[slug]
		if !inBundle {
			if existing, ok := saved[slug]; ok {
				resolved = &existing
			}
		}
		if resolved == nil {
			message := fmt.Sprintf("Không tìm thấy kết nối API %q trong file hoặc trong hệ thống.", slug)
			if inBundle {
				message = fmt.Sprintf("Kết nối API %q trong file không nhập được, nên công cụ này cũng không nhập được.", slug)
			}
			return []domain.FieldError{{Field: "connection_slug", Message: message}}
		}
		connection = resolved
		// Only the presence of a connection matters for validation; the real ID is
		// assigned when the import is written.
		candidate.ConnectionID = &uuid.Nil
	}
	return fieldErrors(s.validateHTTPToolWith(candidate, placeholderSecrets(entry.SecretHeaderNames), connection))
}

func validateBundleShape(bundle domain.ToolBundle) error {
	if bundle.Format != domain.ToolBundleFormat || bundle.Version != domain.ToolBundleVersion {
		return domain.Invalid("format", "File không phải bản xuất công cụ của hệ thống này hoặc thuộc phiên bản không được hỗ trợ.")
	}
	if len(bundle.Connections) > domain.ToolBundleMaxConnections || len(bundle.Tools) > domain.ToolBundleMaxTools {
		return domain.Invalid("tools", fmt.Sprintf("Mỗi file có tối đa %d kết nối API và %d công cụ.", domain.ToolBundleMaxConnections, domain.ToolBundleMaxTools))
	}
	return nil
}

func bundleToolModel(entry domain.ToolBundleTool, connectionID *uuid.UUID) domain.HTTPTool {
	return domain.HTTPTool{ConnectionID: cloneUUIDPointer(connectionID), Slug: entry.Slug, DisplayName: entry.DisplayName, StepLabel: entry.StepLabel, Description: entry.Description, Method: entry.Method, URLTemplate: entry.URLTemplate, Params: cloneParams(entry.Params), PublicHeaders: cloneHeaders(entry.PublicHeaders), TimeoutSeconds: entry.TimeoutSeconds}
}

// placeholderSecrets lets header-name rules run without any secret values.
func placeholderSecrets(names []string) map[string]string {
	result := make(map[string]string, len(names))
	for _, name := range names {
		result[name] = ""
	}
	return result
}

func importStatus(fields []domain.FieldError, exists bool) domain.ToolImportStatus {
	switch {
	case len(fields) > 0:
		return domain.ToolImportInvalid
	case exists:
		return domain.ToolImportConflict
	default:
		return domain.ToolImportNew
	}
}

func duplicateSlugField() []domain.FieldError {
	return []domain.FieldError{{Field: "slug", Message: "Mã này đã xuất hiện ở mục trước trong file; mục lặp lại sẽ được bỏ qua."}}
}

// fieldErrors turns a validation failure into safe per-item details; any other
// failure is unexpected here and is reported generically rather than leaked.
func fieldErrors(err error) []domain.FieldError {
	if err == nil {
		return nil
	}
	var detailed *domain.Error
	if errors.As(err, &detailed) && len(detailed.Fields) > 0 {
		return append([]domain.FieldError{}, detailed.Fields...)
	}
	return []domain.FieldError{{Field: "", Message: "Cấu hình không hợp lệ."}}
}
