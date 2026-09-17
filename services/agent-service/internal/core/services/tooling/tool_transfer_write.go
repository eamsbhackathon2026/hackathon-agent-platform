package tooling

import (
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// Imported entries carry secret header names but never values: a new entry starts with
// no secrets, and an overwritten one keeps the saved values whose names remain.

func (s *Service) writeImportedConnection(ctx context.Context, planned connectionImport) (uuid.UUID, *domain.ToolImportSecretReminder, error) {
	entry, now := planned.entry, s.Clock.Now()
	connection := domain.APIConnection{Slug: entry.Slug, DisplayName: entry.DisplayName, BaseURL: entry.BaseURL, PublicHeaders: cloneHeaders(entry.PublicHeaders), CreatedAt: now, UpdatedAt: now}
	current := map[string]string{}
	var err error
	if planned.existing != nil {
		connection.ID, connection.CreatedAt = planned.existing.ID, planned.existing.CreatedAt
		if current, err = s.decryptHeaders("api_connection", connection.ID, planned.existing.SecretHeadersCiphertext); err != nil {
			return uuid.Nil, nil, err
		}
	} else if connection.ID, err = s.IDs.NewID(); err != nil {
		return uuid.Nil, nil, err
	}
	kept, missing := keepSecrets(current, entry.SecretHeaderNames)
	if connection.SecretHeadersCiphertext, connection.SecretHeaderNames, err = s.encryptHeaders("api_connection", connection.ID, kept); err != nil {
		return uuid.Nil, nil, err
	}
	if planned.existing != nil {
		err = s.Connections.UpdateAPIConnection(ctx, connection)
	} else {
		err = s.Connections.CreateAPIConnection(ctx, connection)
	}
	return connection.ID, secretReminder(planned.item, connection.ID, missing), err
}

func (s *Service) writeImportedTool(ctx context.Context, planned toolImport, connectionIDs map[string]uuid.UUID) (*domain.ToolImportSecretReminder, error) {
	var connectionID *uuid.UUID
	if slug := planned.entry.ConnectionSlug; slug != nil {
		id, ok := connectionIDs[*slug]
		if !ok {
			return nil, errors.New("imported tool connection was not resolved")
		}
		connectionID = &id
	}
	tool, now := bundleToolModel(planned.entry, connectionID), s.Clock.Now()
	tool.CreatedAt, tool.UpdatedAt = now, now
	current := map[string]string{}
	var err error
	if planned.existing != nil {
		tool.ID, tool.CreatedAt = planned.existing.ID, planned.existing.CreatedAt
		if current, err = s.decryptHeaders("tool", tool.ID, planned.existing.SecretHeadersCiphertext); err != nil {
			return nil, err
		}
	} else if tool.ID, err = s.IDs.NewID(); err != nil {
		return nil, err
	}
	kept, missing := keepSecrets(current, planned.entry.SecretHeaderNames)
	if tool.SecretHeadersCiphertext, tool.SecretHeaderNames, err = s.encryptHeaders("tool", tool.ID, kept); err != nil {
		return nil, err
	}
	if planned.existing != nil {
		err = s.Tools.UpdateTool(ctx, tool)
	} else {
		err = s.Tools.CreateTool(ctx, tool)
	}
	return secretReminder(planned.item, tool.ID, missing), err
}

// keepSecrets retains saved values for header names the bundle still declares and
// reports the declared names that have no saved value to reuse.
func keepSecrets(current map[string]string, names []string) (map[string]string, []string) {
	kept := map[string]string{}
	missing := []string{}
	for _, name := range names {
		canonical := canonicalHeaderName(name)
		if _, done := kept[canonical]; done {
			continue
		}
		if value, ok := current[canonical]; ok {
			kept[canonical] = value
		} else if !containsString(missing, canonical) {
			missing = append(missing, canonical)
		}
	}
	sort.Strings(missing)
	return kept, missing
}

func secretReminder(item domain.ToolImportItem, id uuid.UUID, missing []string) *domain.ToolImportSecretReminder {
	if len(missing) == 0 {
		return nil
	}
	return &domain.ToolImportSecretReminder{ToolImportKey: item.ToolImportKey, ID: id, DisplayName: item.DisplayName, HeaderNames: missing}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func countImported(result *domain.ToolImportResult, overwritten bool, reminder *domain.ToolImportSecretReminder) {
	if overwritten {
		result.Overwritten++
	} else {
		result.Created++
	}
	if reminder != nil {
		result.NeedsSecrets = append(result.NeedsSecrets, *reminder)
	}
}
