package http

import (
	"encoding/json"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
)

// The contract declares secret_header_names as a required array. A tool saved
// without secret headers carries a nil slice, which Go would otherwise marshal
// as null and break every client that iterates the field.
func TestToolDTOsEmitEmptyArrayForMissingSecretHeaderNames(t *testing.T) {
	tool, err := json.Marshal(httpToolDTO(domain.HTTPTool{Method: domain.HTTPToolGET, PublicHeaders: map[string]string{}}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tool), `"secret_header_names":[]`) {
		t.Fatalf("http tool DTO phải trả mảng rỗng, nhận: %s", tool)
	}
	server, err := json.Marshal(mcpServerDTO(domain.MCPServer{}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(server), `"secret_header_names":[]`) {
		t.Fatalf("mcp server DTO phải trả mảng rỗng, nhận: %s", server)
	}
}

// show_in_progress declares a default, which the generated TypeScript type reads
// as always present. The DTO must therefore round-trip both true and false rather
// than only carrying the value when it is true.
func TestToolParamsDTORoundTripsShowInProgressBothWays(t *testing.T) {
	params := []domain.ToolParam{
		{Name: "month", Type: domain.ToolParamString, Location: domain.ToolParamQuery, ShowInProgress: true},
		{Name: "recipient_account", Type: domain.ToolParamString, Location: domain.ToolParamBody, ShowInProgress: false},
	}
	dtos := toolParamsDTO(params)
	if len(dtos) != 2 || dtos[0].ShowInProgress == nil || !*dtos[0].ShowInProgress {
		t.Fatalf("show_in_progress=true bị mất: %+v", dtos)
	}
	if dtos[1].ShowInProgress == nil || *dtos[1].ShowInProgress {
		t.Fatalf("show_in_progress=false phải có mặt và là false: %+v", dtos)
	}
	back := toolParams(dtos)
	if len(back) != 2 || !back[0].ShowInProgress || back[1].ShowInProgress {
		t.Fatalf("giải mã ngược show_in_progress sai: %+v", back)
	}
}

// A row saved before this flag existed, or a client that omits the field, must
// decode to false rather than erroring or panicking on a nil pointer.
func TestToolParamsDecodesMissingShowInProgressAsFalse(t *testing.T) {
	back := toolParams([]gen.ToolParam{{Name: "q", Type: "string", In: "query"}})
	if len(back) != 1 || back[0].ShowInProgress {
		t.Fatalf("thiếu show_in_progress phải đọc ra tắt: %+v", back)
	}
}
