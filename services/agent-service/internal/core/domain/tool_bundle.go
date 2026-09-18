package domain

import (
	"time"

	"github.com/google/uuid"
)

// Tool bundle identity. A bundle with any other format or version is rejected so a
// future layout change can never be half-read as the current one.
const (
	ToolBundleFormat         = "agent-platform.tools"
	ToolBundleVersion        = 1
	ToolBundleMaxConnections = 50
	ToolBundleMaxTools       = 200
)

// ToolBundleConnection is a portable API connection: it carries secret header
// names so the importer knows what to re-enter, never their values.
type ToolBundleConnection struct {
	Slug, DisplayName, BaseURL string
	PublicHeaders              map[string]string
	SecretHeaderNames          []string
}

// ToolBundleTool is a portable HTTP tool. It points at its connection by slug
// because connection IDs differ between environments.
type ToolBundleTool struct {
	Slug, DisplayName, Description string
	StepLabel                      string
	Method                         HTTPToolMethod
	URLTemplate                    string
	ConnectionSlug                 *string
	Params                         []ToolParam
	PublicHeaders                  map[string]string
	TimeoutSeconds                 int
	SecretHeaderNames              []string
}

// ToolBundle is the versioned export/import document for HTTP tool configuration.
type ToolBundle struct {
	Format      string
	Version     int
	ExportedAt  time.Time
	Connections []ToolBundleConnection
	Tools       []ToolBundleTool
}

// ToolImportKind names which kind of bundle entry an import item describes.
type ToolImportKind string

// Bundle entry kinds.
const (
	ToolImportConnection ToolImportKind = "connection"
	ToolImportTool       ToolImportKind = "tool"
)

// ToolImportStatus classifies one bundle entry against the saved configuration.
type ToolImportStatus string

// Import item statuses.
const (
	ToolImportNew      ToolImportStatus = "new"
	ToolImportConflict ToolImportStatus = "conflict"
	ToolImportInvalid  ToolImportStatus = "invalid"
)

// ToolImportAction is the user's choice for an entry whose slug already exists.
type ToolImportAction string

// Import actions for conflicting entries.
const (
	ToolImportSkip      ToolImportAction = "skip"
	ToolImportOverwrite ToolImportAction = "overwrite"
)

// ToolImportKey identifies one bundle entry; slugs are unique per kind.
type ToolImportKey struct {
	Kind ToolImportKind
	Slug string
}

// ToolImportItem is the preview outcome for one bundle entry.
type ToolImportItem struct {
	ToolImportKey
	DisplayName string
	Status      ToolImportStatus
	Fields      []FieldError
}

// ToolImportSecretReminder lists secret headers that still need values after import.
type ToolImportSecretReminder struct {
	ToolImportKey
	ID          uuid.UUID
	DisplayName string
	HeaderNames []string
}

// ToolImportResult summarizes a committed import.
type ToolImportResult struct {
	Created, Overwritten, Skipped int
	NeedsSecrets                  []ToolImportSecretReminder
}
