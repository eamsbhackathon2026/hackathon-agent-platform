package tooling

import (
	"testing"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

// A resolved ToolSpec is the only thing the run engine consults when deciding
// which arguments it may surface in tool.started, so it must carry exactly the
// parameters an operator marked show_in_progress and nothing else.
func TestResolveCarriesOnlyTheParamsAnOperatorMarkedVisible(t *testing.T) {
	fixture := newServiceFixture(t)
	command := validToolCommand()
	command.Params = []domain.ToolParam{
		{Name: "city", Type: domain.ToolParamString, Required: true, Location: domain.ToolParamPath, ShowInProgress: true},
		{Name: "unit", Type: domain.ToolParamString, Location: domain.ToolParamQuery},
	}
	tool, err := fixture.service.CreateTool(t.Context(), fixture.admin, command)
	if err != nil {
		t.Fatal(err)
	}
	bindings := domain.AgentToolBindings{ToolIDs: []uuid.UUID{tool.ID}}
	if _, err = fixture.service.ReplaceAgentTools(t.Context(), fixture.admin, fixture.agentID, bindings); err != nil {
		t.Fatal(err)
	}
	set, err := fixture.service.Resolve(t.Context(), fixture.agentID)
	if err != nil {
		t.Fatal(err)
	}
	specs := set.Specs()
	if len(specs) != 1 {
		t.Fatalf("specs=%d", len(specs))
	}
	if got := specs[0].VisibleParams; len(got) != 1 || got[0] != "city" {
		t.Fatalf("visible params=%v, want only city", got)
	}
}

// A tool that opts no parameter in must resolve to an empty, not nil, list so the
// run engine's lookup never has to special-case "no spec found" against "spec
// found with nothing visible".
func TestResolveCarriesNoVisibleParamsWhenNoneAreMarked(t *testing.T) {
	fixture := newServiceFixture(t)
	tool, err := fixture.service.CreateTool(t.Context(), fixture.admin, validToolCommand())
	if err != nil {
		t.Fatal(err)
	}
	bindings := domain.AgentToolBindings{ToolIDs: []uuid.UUID{tool.ID}}
	if _, err = fixture.service.ReplaceAgentTools(t.Context(), fixture.admin, fixture.agentID, bindings); err != nil {
		t.Fatal(err)
	}
	set, err := fixture.service.Resolve(t.Context(), fixture.agentID)
	if err != nil {
		t.Fatal(err)
	}
	specs := set.Specs()
	if len(specs) != 1 || specs[0].VisibleParams == nil || len(specs[0].VisibleParams) != 0 {
		t.Fatalf("visible params=%v", specs[0].VisibleParams)
	}
}
