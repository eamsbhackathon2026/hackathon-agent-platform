package http

import (
	"encoding/json"
	"strings"
	"testing"

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
