//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
)

func toolingFixture(t *testing.T) (*Store, domain.HTTPTool, domain.MCPServer, domain.Agent) {
	t.Helper()
	store, provider, agent := catalogFixture(t)
	ctx := context.Background()
	if err := store.CreateProvider(ctx, provider); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	tool := domain.HTTPTool{ID: uuid.New(), Slug: "weather", DisplayName: "Thời tiết", StepLabel: "Đang xem thời tiết", Description: "Tra cứu", Method: domain.HTTPToolGET, URLTemplate: "https://api.example.com/{city}", Params: []domain.ToolParam{{Name: "city", Type: domain.ToolParamString, Required: true, Location: domain.ToolParamPath}}, PublicHeaders: map[string]string{"Accept": "application/json"}, SecretHeadersCiphertext: []byte{0, 1, 2}, SecretHeaderNames: []string{"Authorization"}, TimeoutSeconds: 15, CreatedAt: now, UpdatedAt: now}
	server := domain.MCPServer{ID: uuid.New(), Slug: "office", DisplayName: "Văn phòng", URL: "https://mcp.example.com", SecretHeadersCiphertext: []byte{3, 4}, SecretHeaderNames: []string{"Authorization"}, Tools: []domain.MCPTool{{Name: "search", Description: "Tìm", InputSchema: []byte(`{"type":"object"}`)}}, Status: domain.ConnectionUnchecked, CreatedAt: now, UpdatedAt: now, Revision: 1}
	return store, tool, server, agent
}

func TestToolingRepositoriesRoundTripAndCAS(t *testing.T) {
	store, tool, server, _ := toolingFixture(t)
	ctx := context.Background()
	if err := store.CreateTool(ctx, tool); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMCPServer(ctx, server); err != nil {
		t.Fatal(err)
	}
	gotTool, err := store.GetTool(ctx, tool.ID)
	if err != nil || gotTool.Params[0].Name != "city" || gotTool.PublicHeaders["Accept"] != "application/json" || len(gotTool.SecretHeadersCiphertext) != 3 || gotTool.StepLabel != "Đang xem thời tiết" {
		t.Fatalf("tool=%+v err=%v", gotTool, err)
	}
	gotServer, err := store.GetMCPServer(ctx, server.ID)
	if err != nil || gotServer.AllowedTools != nil || len(gotServer.Tools) != 1 || gotServer.Revision != 1 {
		t.Fatalf("server=%+v err=%v", gotServer, err)
	}
	tool.DisplayName = "Mới"
	tool.StepLabel = "Đang xem dự báo"
	tool.UpdatedAt = tool.UpdatedAt.Add(time.Second)
	if err = store.UpdateTool(ctx, tool); err != nil {
		t.Fatal(err)
	}
	// Cột được thêm sau nên dễ lọt khỏi câu UPDATE: ghi xong đọc lại mới biết.
	if updated, updateErr := store.GetTool(ctx, tool.ID); updateErr != nil || updated.StepLabel != "Đang xem dự báo" {
		t.Fatalf("nhãn bước sau khi sửa=%q err=%v", updated.StepLabel, updateErr)
	}
	// Công cụ lưu trước khi có cột này đọc ra chuỗi rỗng chứ không phải NULL, nhờ
	// DEFAULT '' của migration; bên gọi vì thế rơi về tên hiển thị thay vì vỡ.
	bare := tool
	bare.ID, bare.Slug, bare.StepLabel = uuid.New(), "bare", ""
	if err = store.CreateTool(ctx, bare); err != nil {
		t.Fatal(err)
	}
	if got, bareErr := store.GetTool(ctx, bare.ID); bareErr != nil || got.StepLabel != "" {
		t.Fatalf("nhãn bước của công cụ chưa đặt=%q err=%v", got.StepLabel, bareErr)
	}
	empty := []string{}
	gotServer.AllowedTools = &empty
	gotServer.DisplayName = "Mới"
	gotServer, err = store.UpdateMCPServer(ctx, gotServer)
	if err != nil || gotServer.Revision != 2 || gotServer.AllowedTools == nil || len(*gotServer.AllowedTools) != 0 {
		t.Fatalf("updated server=%+v err=%v", gotServer, err)
	}
	failure := &domain.ProviderFailure{Code: "tool_failed", Message: "Không thể kết nối"}
	applied, err := store.RecordMCPServerSync(ctx, server.ID, 1, nil, domain.ConnectionFailing, failure, time.Now())
	if err != nil || applied {
		t.Fatalf("stale applied=%v err=%v", applied, err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	applied, err = store.RecordMCPServerSync(ctx, server.ID, 2, server.Tools, domain.ConnectionOK, nil, at)
	if err != nil || !applied {
		t.Fatalf("sync applied=%v err=%v", applied, err)
	}
	gotServer, _ = store.GetMCPServer(ctx, server.ID)
	if gotServer.Status != domain.ConnectionOK || gotServer.LastSyncedAt == nil || !gotServer.LastSyncedAt.Equal(at) {
		t.Fatalf("synced server=%+v", gotServer)
	}
	tools, err := store.ListTools(ctx, domain.PageOptions{Limit: 1})
	if err != nil || len(tools) != 1 {
		t.Fatalf("tools=%+v err=%v", tools, err)
	}
	servers, err := store.ListMCPServers(ctx, domain.PageOptions{Limit: 1})
	if err != nil || len(servers) != 1 {
		t.Fatalf("servers=%+v err=%v", servers, err)
	}
}

func TestAPIConnectionRepositoryReferenceProtection(t *testing.T) {
	store, tool, _, _ := toolingFixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	connection := domain.APIConnection{ID: uuid.New(), Slug: "orders", DisplayName: "Orders API", BaseURL: "https://api.example.com/v1", PublicHeaders: map[string]string{"Accept": "application/json"}, SecretHeadersCiphertext: []byte{1, 2, 3}, SecretHeaderNames: []string{"Authorization"}, CreatedAt: now, UpdatedAt: now}
	if err := store.CreateAPIConnection(ctx, connection); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetAPIConnection(ctx, connection.ID)
	if err != nil || got.BaseURL != connection.BaseURL || got.PublicHeaders["Accept"] != "application/json" || len(got.SecretHeadersCiphertext) != 3 {
		t.Fatalf("connection=%+v err=%v", got, err)
	}
	tool.ConnectionID = &connection.ID
	tool.URLTemplate = "/weather/{city}"
	if err = store.CreateTool(ctx, tool); err != nil {
		t.Fatal(err)
	}
	gotTool, err := store.GetTool(ctx, tool.ID)
	if err != nil || gotTool.ConnectionID == nil || *gotTool.ConnectionID != connection.ID {
		t.Fatalf("tool=%+v err=%v", gotTool, err)
	}
	count, err := store.CountToolsByAPIConnection(ctx, connection.ID)
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err = store.DeleteAPIConnection(ctx, connection.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("foreign-key delete err=%v", err)
	}
	if err = store.DeleteTool(ctx, tool.ID); err != nil {
		t.Fatal(err)
	}
	connection.BaseURL = "https://api.example.com/v2"
	connection.UpdatedAt = now.Add(time.Second)
	if err = store.UpdateAPIConnection(ctx, connection); err != nil {
		t.Fatal(err)
	}
	connections, err := store.ListAPIConnections(ctx, domain.PageOptions{Limit: 1})
	if err != nil || len(connections) != 1 || connections[0].BaseURL != connection.BaseURL {
		t.Fatalf("connections=%+v err=%v", connections, err)
	}
	if err = store.DeleteAPIConnection(ctx, connection.ID); err != nil {
		t.Fatal(err)
	}
	empty := domain.APIConnection{ID: uuid.New(), Slug: "public-api", DisplayName: "Public API", BaseURL: "https://public.example.com", PublicHeaders: map[string]string{}, SecretHeaderNames: []string{}, CreatedAt: now, UpdatedAt: now}
	if err = store.CreateAPIConnection(ctx, empty); err != nil {
		t.Fatal(err)
	}
	gotEmpty, err := store.GetAPIConnection(ctx, empty.ID)
	if err != nil || gotEmpty.SecretHeaderNames == nil {
		t.Fatalf("empty secret names=%#v err=%v", gotEmpty.SecretHeaderNames, err)
	}
	if err = store.DeleteAPIConnection(ctx, empty.ID); err != nil {
		t.Fatal(err)
	}
}

func TestToolBindingsReplaceRollbackResolveAndCascade(t *testing.T) {
	store, tool, server, agent := toolingFixture(t)
	ctx := context.Background()
	connection := domain.APIConnection{ID: uuid.New(), Slug: "weather-api", DisplayName: "Weather API", BaseURL: "https://api.example.com/v1", PublicHeaders: map[string]string{"Accept": "application/json"}, SecretHeadersCiphertext: []byte{1, 2, 3}, SecretHeaderNames: []string{"Authorization"}, CreatedAt: tool.CreatedAt, UpdatedAt: tool.UpdatedAt}
	if err := store.CreateAPIConnection(ctx, connection); err != nil {
		t.Fatal(err)
	}
	tool.ConnectionID = &connection.ID
	tool.URLTemplate = "/weather/{city}"
	if err := store.CreateTool(ctx, tool); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateMCPServer(ctx, server); err != nil {
		t.Fatal(err)
	}
	bindings := domain.AgentToolBindings{ToolIDs: []uuid.UUID{tool.ID}, MCPServerIDs: []uuid.UUID{server.ID}}
	if err := store.WithinTx(ctx, func(ctx context.Context) error {
		if err := store.LockTooling(ctx); err != nil {
			return err
		}
		return store.ReplaceAgentToolBindings(ctx, agent.ID, bindings)
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err := store.ResolveAgentTools(ctx, agent.ID)
	if err != nil || len(catalog.HTTPTools) != 1 || len(catalog.APIConnections) != 1 || len(catalog.MCPServers) != 1 {
		t.Fatalf("resolved=%+v err=%v", catalog, err)
	}
	if catalog.HTTPTools[0].ConnectionID == nil || *catalog.HTTPTools[0].ConnectionID != connection.ID || catalog.APIConnections[0].BaseURL != connection.BaseURL {
		t.Fatalf("connection snapshot=%+v", catalog)
	}
	err = store.WithinTx(ctx, func(ctx context.Context) error {
		if err := store.LockTooling(ctx); err != nil {
			return err
		}
		return store.ReplaceAgentToolBindings(ctx, agent.ID, domain.AgentToolBindings{ToolIDs: []uuid.UUID{uuid.New()}})
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("invalid replacement err=%v", err)
	}
	saved, err := store.GetAgentToolBindings(ctx, agent.ID)
	if err != nil || len(saved.ToolIDs) != 1 || len(saved.MCPServerIDs) != 1 {
		t.Fatalf("rollback bindings=%+v err=%v", saved, err)
	}
	if err = store.DeleteTool(ctx, tool.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.DeleteMCPServer(ctx, server.ID); err != nil {
		t.Fatal(err)
	}
	if err = store.DeleteAPIConnection(ctx, connection.ID); err != nil {
		t.Fatal(err)
	}
	saved, err = store.GetAgentToolBindings(ctx, agent.ID)
	if err != nil || len(saved.ToolIDs) != 0 || len(saved.MCPServerIDs) != 0 {
		t.Fatalf("cascade bindings=%+v err=%v", saved, err)
	}
	if err = store.LockTooling(ctx); err == nil {
		t.Fatal("lock outside transaction succeeded")
	}
}
