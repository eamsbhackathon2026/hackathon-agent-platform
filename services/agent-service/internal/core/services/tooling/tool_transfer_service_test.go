package tooling

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

type transferDecisions = map[domain.ToolImportKey]domain.ToolImportAction

func connectedToolFixture(t *testing.T) (serviceFixture, domain.APIConnection, domain.HTTPTool) {
	t.Helper()
	fixture := newServiceFixture(t)
	connection, err := fixture.service.CreateAPIConnection(t.Context(), fixture.admin, validAPIConnectionCommand())
	if err != nil {
		t.Fatal(err)
	}
	command := validToolCommand()
	command.ConnectionID = &connection.ID
	command.URLTemplate = "/weather/{city}"
	// The step label travels in the bundle: losing it on the way to another
	// environment means an operator retypes every label by hand, and until they do,
	// customers read the catalog name.
	command.StepLabel = "Đang xem thời tiết"
	tool, err := fixture.service.CreateTool(t.Context(), fixture.admin, command)
	if err != nil {
		t.Fatal(err)
	}
	return fixture, connection, tool
}

func toolKey(slug string) domain.ToolImportKey {
	return domain.ToolImportKey{Kind: domain.ToolImportTool, Slug: slug}
}

func connectionKey(slug string) domain.ToolImportKey {
	return domain.ToolImportKey{Kind: domain.ToolImportConnection, Slug: slug}
}

func statuses(items []domain.ToolImportItem) map[domain.ToolImportKey]domain.ToolImportStatus {
	result := map[domain.ToolImportKey]domain.ToolImportStatus{}
	for _, item := range items {
		result[item.ToolImportKey] = item.Status
	}
	return result
}

func TestExportToolsCarriesConnectionSlugsWithoutSecrets(t *testing.T) {
	fixture, _, _ := connectedToolFixture(t)
	direct := validToolCommand()
	direct.Slug = "direct"
	if _, err := fixture.service.CreateTool(t.Context(), fixture.admin, direct); err != nil {
		t.Fatal(err)
	}
	unused := validAPIConnectionCommand()
	unused.Slug = "unused"
	if _, err := fixture.service.CreateAPIConnection(t.Context(), fixture.admin, unused); err != nil {
		t.Fatal(err)
	}
	bundle, err := fixture.service.ExportTools(t.Context(), fixture.member)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Format != domain.ToolBundleFormat || bundle.Version != 1 || len(bundle.Tools) != 2 || len(bundle.Connections) != 1 || bundle.Connections[0].Slug != "orders" {
		t.Fatalf("bundle=%+v", bundle)
	}
	encoded, _ := json.Marshal(bundle)
	for _, leaked := range []string{"Bearer private", "Bearer shared"} {
		if strings.Contains(string(encoded), leaked) {
			t.Fatalf("export leaked a secret value: %s", encoded)
		}
	}
	for _, tool := range bundle.Tools {
		switch tool.Slug {
		case "weather":
			if tool.ConnectionSlug == nil || *tool.ConnectionSlug != "orders" || !reflect.DeepEqual(tool.SecretHeaderNames, []string{"Authorization"}) {
				t.Fatalf("connected tool=%+v", tool)
			}
		case "direct":
			if tool.ConnectionSlug != nil {
				t.Fatalf("direct tool=%+v", tool)
			}
		}
	}
	if _, err = fixture.service.ExportTools(t.Context(), domain.Principal{}); err == nil {
		t.Fatal("anonymous export accepted")
	}
}

func TestExportThenImportRecreatesConfigurationInEmptyEnvironment(t *testing.T) {
	source, _, _ := connectedToolFixture(t)
	bundle, err := source.service.ExportTools(t.Context(), source.admin)
	if err != nil {
		t.Fatal(err)
	}
	target := newServiceFixture(t)
	items, err := target.service.PreviewToolImport(t.Context(), target.admin, bundle)
	if err != nil {
		t.Fatal(err)
	}
	want := map[domain.ToolImportKey]domain.ToolImportStatus{connectionKey("orders"): domain.ToolImportNew, toolKey("weather"): domain.ToolImportNew}
	if got := statuses(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("preview=%v", got)
	}
	if tools, _ := target.store.ListAllTools(t.Context()); len(tools) != 0 {
		t.Fatal("preview wrote data")
	}
	result, err := target.service.ImportTools(t.Context(), target.admin, bundle, nil)
	if err != nil || result.Created != 2 || result.Overwritten != 0 || result.Skipped != 0 || len(result.NeedsSecrets) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	connections, _ := target.store.ListAllAPIConnections(t.Context())
	tools, _ := target.store.ListAllTools(t.Context())
	if len(connections) != 1 || len(tools) != 1 || tools[0].ConnectionID == nil || *tools[0].ConnectionID != connections[0].ID {
		t.Fatalf("connections=%+v tools=%+v", connections, tools)
	}
	if len(tools[0].SecretHeaderNames) != 0 || tools[0].URLTemplate != "/weather/{city}" || tools[0].PublicHeaders["Accept"] != "application/json" {
		t.Fatalf("imported tool=%+v", tools[0])
	}
	again, err := target.service.ExportTools(t.Context(), target.admin)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Tools[0].SecretHeaderNames, bundle.Connections[0].SecretHeaderNames = []string{}, []string{}
	again.ExportedAt = bundle.ExportedAt
	if !reflect.DeepEqual(again, bundle) {
		t.Fatalf("round trip changed configuration:\n got=%+v\nwant=%+v", again, bundle)
	}
}

func TestImportConflictsNeedDecisionsAndOverwriteKeepsSecrets(t *testing.T) {
	fixture, connection, tool := connectedToolFixture(t)
	bundle, err := fixture.service.ExportTools(t.Context(), fixture.admin)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Tools[0].DisplayName = "Thời tiết mới"
	bundle.Tools[0].SecretHeaderNames = []string{"authorization", "X-Extra"}
	items, err := fixture.service.PreviewToolImport(t.Context(), fixture.admin, bundle)
	if err != nil || statuses(items)[toolKey("weather")] != domain.ToolImportConflict || statuses(items)[connectionKey("orders")] != domain.ToolImportConflict {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if _, err = fixture.service.ImportTools(t.Context(), fixture.admin, bundle, nil); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("undecided conflict err=%v", err)
	}
	var detailed *domain.Error
	if !errors.As(err, &detailed) || len(detailed.Fields) != 2 || detailed.Fields[0].Field != "connections.orders" {
		t.Fatalf("detail=%+v", detailed)
	}
	decisions := transferDecisions{connectionKey("orders"): domain.ToolImportSkip, toolKey("weather"): domain.ToolImportOverwrite}
	result, err := fixture.service.ImportTools(t.Context(), fixture.admin, bundle, decisions)
	if err != nil || result.Overwritten != 1 || result.Skipped != 1 || result.Created != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(result.NeedsSecrets) != 1 || result.NeedsSecrets[0].ID != tool.ID || !reflect.DeepEqual(result.NeedsSecrets[0].HeaderNames, []string{"X-Extra"}) {
		t.Fatalf("reminders=%+v", result.NeedsSecrets)
	}
	saved, err := fixture.store.GetTool(t.Context(), tool.ID)
	if err != nil || saved.DisplayName != "Thời tiết mới" || !saved.CreatedAt.Equal(tool.CreatedAt) || *saved.ConnectionID != connection.ID {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	secrets, err := fixture.service.decryptHeaders("tool", tool.ID, saved.SecretHeadersCiphertext)
	if err != nil || !reflect.DeepEqual(secrets, map[string]string{"Authorization": "Bearer private"}) {
		t.Fatalf("secrets=%v err=%v", secrets, err)
	}
}

func TestImportIsAtomicAndReportsInvalidEntries(t *testing.T) {
	fixture := newServiceFixture(t)
	missing := "nowhere"
	good := domain.ToolBundleTool{Slug: "good", DisplayName: "Good", Method: domain.HTTPToolGET, URLTemplate: "https://api.example.com/good", PublicHeaders: map[string]string{}, TimeoutSeconds: 15}
	orphan := good
	orphan.Slug, orphan.ConnectionSlug, orphan.URLTemplate = "orphan", &missing, "/orphan"
	broken := good
	broken.Slug, broken.URLTemplate = "broken", "https://api.example.com/{id}"
	bundle := domain.ToolBundle{Format: domain.ToolBundleFormat, Version: 1, Tools: []domain.ToolBundleTool{good, orphan, broken, good}}
	items, err := fixture.service.PreviewToolImport(t.Context(), fixture.admin, bundle)
	if err != nil || len(items) != 4 {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if items[0].Status != domain.ToolImportNew || items[1].Fields[0].Field != "connection_slug" || items[2].Fields[0].Field != "url_template" || items[3].Status != domain.ToolImportInvalid {
		t.Fatalf("items=%+v", items)
	}
	if _, err = fixture.service.ImportTools(t.Context(), fixture.admin, bundle, nil); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("invalid import err=%v", err)
	}
	if tools, _ := fixture.store.ListAllTools(t.Context()); len(tools) != 0 {
		t.Fatalf("partial import wrote %d tools", len(tools))
	}
	decisions := transferDecisions{toolKey("orphan"): domain.ToolImportSkip, toolKey("broken"): domain.ToolImportSkip}
	result, err := fixture.service.ImportTools(t.Context(), fixture.admin, bundle, decisions)
	if err != nil || result.Created != 1 || result.Skipped != 3 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestImportToolFollowsSkippedBundleConnectionToSavedOne(t *testing.T) {
	fixture, connection, _ := connectedToolFixture(t)
	bundle, err := fixture.service.ExportTools(t.Context(), fixture.admin)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Connections[0].BaseURL = "not a url"
	bundle.Tools[0].Slug = "weather-copy"
	items, err := fixture.service.PreviewToolImport(t.Context(), fixture.admin, bundle)
	// The unusable bundle connection is skipped on import, so the tool falls back to the saved one.
	if err != nil || statuses(items)[connectionKey("orders")] != domain.ToolImportInvalid || statuses(items)[toolKey("weather-copy")] != domain.ToolImportNew {
		t.Fatalf("preview=%+v err=%v", items, err)
	}
	decisions := transferDecisions{connectionKey("orders"): domain.ToolImportSkip}
	result, err := fixture.service.ImportTools(t.Context(), fixture.admin, bundle, decisions)
	if err != nil || result.Created != 1 || result.Skipped != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	tools, _ := fixture.store.ListAllTools(t.Context())
	for _, tool := range tools {
		if tool.Slug == "weather-copy" && (tool.ConnectionID == nil || *tool.ConnectionID != connection.ID) {
			t.Fatalf("copy is not bound to the saved connection: %+v", tool)
		}
	}
	saved, _ := fixture.store.GetAPIConnection(t.Context(), connection.ID)
	if saved.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("skipped connection changed: %+v", saved)
	}
}

func TestImportRejectsForeignBundlesAndMembers(t *testing.T) {
	fixture := newServiceFixture(t)
	valid := domain.ToolBundle{Format: domain.ToolBundleFormat, Version: 1}
	for name, bundle := range map[string]domain.ToolBundle{
		"format":      {Format: "other", Version: 1},
		"version":     {Format: domain.ToolBundleFormat, Version: 2},
		"tools":       {Format: domain.ToolBundleFormat, Version: 1, Tools: make([]domain.ToolBundleTool, domain.ToolBundleMaxTools+1)},
		"connections": {Format: domain.ToolBundleFormat, Version: 1, Connections: make([]domain.ToolBundleConnection, domain.ToolBundleMaxConnections+1)},
	} {
		if _, err := fixture.service.PreviewToolImport(t.Context(), fixture.admin, bundle); !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("%s err=%v", name, err)
		}
	}
	if _, err := fixture.service.PreviewToolImport(t.Context(), fixture.member, valid); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member preview err=%v", err)
	}
	if _, err := fixture.service.ImportTools(t.Context(), fixture.member, valid, nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member import err=%v", err)
	}
}

func TestImportToolUsesSavedConnectionMissingFromBundle(t *testing.T) {
	fixture, connection, _ := connectedToolFixture(t)
	bundle, err := fixture.service.ExportTools(t.Context(), fixture.admin)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Connections = nil
	bundle.Tools[0].Slug = "weather-copy"
	items, err := fixture.service.PreviewToolImport(t.Context(), fixture.admin, bundle)
	if err != nil || statuses(items)[toolKey("weather-copy")] != domain.ToolImportNew {
		t.Fatalf("preview=%+v err=%v", items, err)
	}
	result, err := fixture.service.ImportTools(t.Context(), fixture.admin, bundle, nil)
	if err != nil || result.Created != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	tools, _ := fixture.store.ListAllTools(t.Context())
	for _, tool := range tools {
		if tool.Slug == "weather-copy" && (tool.ConnectionID == nil || *tool.ConnectionID != connection.ID) {
			t.Fatalf("copy is not bound to the saved connection: %+v", tool)
		}
	}
}

func TestExportRefusesMoreToolsThanImportAccepts(t *testing.T) {
	fixture := newServiceFixture(t)
	for i := 0; i <= domain.ToolBundleMaxTools; i++ {
		command := validToolCommand()
		command.Slug = fmt.Sprintf("tool-%d", i)
		if _, err := fixture.service.CreateTool(t.Context(), fixture.admin, command); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fixture.service.ExportTools(t.Context(), fixture.admin); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("oversized export err=%v", err)
	}
}
