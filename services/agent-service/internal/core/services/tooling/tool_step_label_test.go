package tooling

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// The resolver is where saved configuration and the provider-facing name meet, so
// it is the only place that can decide the label; the run engine just repeats it.
func TestResolveCarriesTheConfiguredStepLabelIntoSpecs(t *testing.T) {
	fixture := newServiceFixture(t)
	labelled := validToolCommand()
	labelled.StepLabel = "Đang xem thời tiết"
	withLabel, err := fixture.service.CreateTool(t.Context(), fixture.admin, labelled)
	if err != nil {
		t.Fatal(err)
	}
	bare := validToolCommand()
	bare.Slug = "forecast"
	bare.DisplayName = "Dự báo"
	withoutLabel, err := fixture.service.CreateTool(t.Context(), fixture.admin, bare)
	if err != nil {
		t.Fatal(err)
	}
	server, err := fixture.service.CreateMCPServer(t.Context(), fixture.admin, validMCPCommand())
	if err != nil {
		t.Fatal(err)
	}
	fixture.mcp.session = &fakeMCPSession{tools: []domain.MCPTool{{Name: "search", Description: "Tìm", InputSchema: json.RawMessage(`{"type":"object"}`)}}}
	if _, err = fixture.service.RefreshMCPServer(t.Context(), fixture.admin, server.ID); err != nil {
		t.Fatal(err)
	}
	bindings := domain.AgentToolBindings{ToolIDs: []uuid.UUID{withLabel.ID, withoutLabel.ID}, MCPServerIDs: []uuid.UUID{server.ID}}
	if _, err = fixture.service.ReplaceAgentTools(t.Context(), fixture.admin, fixture.agentID, bindings); err != nil {
		t.Fatal(err)
	}
	set, err := fixture.service.Resolve(t.Context(), fixture.agentID)
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]string{}
	for _, spec := range set.Specs() {
		labels[spec.Ref] = spec.DisplayName
	}
	if labels["weather"] != "Đang xem thời tiết" {
		t.Fatalf("step label never reached the spec: %q", labels["weather"])
	}
	// With no step label the catalog name still reads acceptably, as long as it does
	// not fall through to the provider-facing name.
	if labels["forecast"] != "Dự báo" {
		t.Fatalf("missing step label must fall back to the display name, got %q", labels["forecast"])
	}
	// domain.MCPTool carries no label, so the server name is the closest thing to one.
	if labels["office.search"] != "Văn phòng" {
		t.Fatalf("an MCP tool must carry the server name, got %q", labels["office.search"])
	}
}
