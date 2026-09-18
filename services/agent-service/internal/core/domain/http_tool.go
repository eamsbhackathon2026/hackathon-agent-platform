package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// HTTPToolMethod is an outbound HTTP method supported by saved tools.
type HTTPToolMethod string

// Supported HTTP tool request methods.
const (
	HTTPToolGET    HTTPToolMethod = "GET"
	HTTPToolPOST   HTTPToolMethod = "POST"
	HTTPToolPUT    HTTPToolMethod = "PUT"
	HTTPToolPATCH  HTTPToolMethod = "PATCH"
	HTTPToolDELETE HTTPToolMethod = "DELETE"
)

// ToolParamLocation controls how an argument is placed in an HTTP request.
type ToolParamLocation string

// Supported HTTP argument locations.
const (
	ToolParamPath  ToolParamLocation = "path"
	ToolParamQuery ToolParamLocation = "query"
	ToolParamBody  ToolParamLocation = "body"
)

// ToolParamType is the JSON type accepted for a declared argument.
type ToolParamType string

// Supported argument types. Object and array carry structured JSON straight into a
// request body; path and query arguments must be representable as a single string.
const (
	ToolParamString  ToolParamType = "string"
	ToolParamNumber  ToolParamType = "number"
	ToolParamInteger ToolParamType = "integer"
	ToolParamBoolean ToolParamType = "boolean"
	ToolParamObject  ToolParamType = "object"
	ToolParamArray   ToolParamType = "array"
)

// Structured reports whether the type carries nested JSON rather than a scalar.
func (t ToolParamType) Structured() bool {
	return t == ToolParamObject || t == ToolParamArray
}

// ToolParamField declares one member of an object parameter. Fields never nest further.
type ToolParamField struct {
	Name, Description string
	Type              ToolParamType
	Required          bool
}

// ToolParam is the product-facing declaration used to generate JSON Schema.
// Fields applies to object parameters and ItemType to array parameters; both are
// optional, and leaving them empty declares a value of unconstrained shape.
type ToolParam struct {
	Name, Description string
	Type              ToolParamType
	Required          bool
	Location          ToolParamLocation
	Fields            []ToolParamField
	ItemType          ToolParamType
	// ShowInProgress opts this parameter's value into the tool.started event while a
	// call runs. An operator sets it per parameter because a call's arguments can
	// carry data such as an account number or a transfer amount; false is the safe
	// default, and a row saved before this flag existed reads as false.
	ShowInProgress bool
}

// HTTPTool stores encrypted request headers and never exposes their values in DTOs.
type HTTPTool struct {
	ID                uuid.UUID
	ConnectionID      *uuid.UUID
	Slug, DisplayName string
	// StepLabel is what an end user reads while this tool runs; empty means the
	// caller falls back to DisplayName.
	StepLabel               string
	Description             string
	Method                  HTTPToolMethod
	URLTemplate             string
	Params                  []ToolParam
	PublicHeaders           map[string]string
	SecretHeadersCiphertext []byte
	SecretHeaderNames       []string
	TimeoutSeconds          int
	CreatedAt, UpdatedAt    time.Time
}

// HTTPToolInvocation is the normalized result consumed by tests and the run engine.
type HTTPToolInvocation struct {
	StatusCode *int
	Body       string
	Truncated  bool
	IsError    bool
}

// ToolArguments decodes an object while preserving JSON number precision.
func ToolArguments(raw json.RawMessage) (map[string]any, error) {
	return decodeToolArguments(raw)
}
