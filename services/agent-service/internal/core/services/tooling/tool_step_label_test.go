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
		t.Fatalf("nhãn bước không tới spec: %q", labels["weather"])
	}
	// Chưa đặt nhãn bước thì tên trong danh mục vẫn đọc được, miễn là không rơi
	// xuống tên kỹ thuật.
	if labels["forecast"] != "Dự báo" {
		t.Fatalf("thiếu nhãn bước phải rơi về tên hiển thị, nhận %q", labels["forecast"])
	}
	// domain.MCPTool không mang nhãn nào, nên tên server là thứ gần người đọc nhất.
	if labels["office.search"] != "Văn phòng" {
		t.Fatalf("tool MCP phải mang tên server, nhận %q", labels["office.search"])
	}
}
