package tooling

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
	"agent-platform/services/agent-service/internal/testsupport/fakes"
)

type urlPolicyFunc func(string) error

func (f urlPolicyFunc) ValidateURL(value string) error { return f(value) }

type countingCipher struct {
	base fakes.Cipher
	mu   sync.Mutex
	aads []string
}

func (c *countingCipher) Encrypt(plain, aad []byte) ([]byte, error) {
	return c.base.Encrypt(plain, aad)
}

func (c *countingCipher) Decrypt(ciphertext, aad []byte) ([]byte, error) {
	c.mu.Lock()
	c.aads = append(c.aads, string(aad))
	c.mu.Unlock()
	return c.base.Decrypt(ciphertext, aad)
}

func (c *countingCipher) reset() {
	c.mu.Lock()
	c.aads = nil
	c.mu.Unlock()
}

func (c *countingCipher) countPrefix(prefix string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, aad := range c.aads {
		if strings.HasPrefix(aad, prefix) {
			count++
		}
	}
	return count
}

type fakeHTTPInvoker struct {
	mu         sync.Mutex
	invocation domain.HTTPToolInvocation
	err        error
	tool       domain.HTTPTool
	secrets    map[string]string
	args       json.RawMessage
}

func (f *fakeHTTPInvoker) Invoke(_ context.Context, tool domain.HTTPTool, secrets map[string]string, args json.RawMessage) (domain.HTTPToolInvocation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tool, f.secrets, f.args = tool, cloneHeaders(secrets), append(json.RawMessage(nil), args...)
	return f.invocation, f.err
}

type fakeMCPSession struct {
	mu      sync.Mutex
	tools   []domain.MCPTool
	listErr error
	result  domain.ToolResult
	callErr error
	calls   []string
	closed  bool
}

func (s *fakeMCPSession) ListTools(context.Context) ([]domain.MCPTool, error) {
	return append([]domain.MCPTool{}, s.tools...), s.listErr
}

func (s *fakeMCPSession) CallTool(_ context.Context, name string, _ json.RawMessage) (domain.ToolResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, name)
	return s.result, s.callErr
}

func (s *fakeMCPSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

type fakeMCPConnector struct {
	mu       sync.Mutex
	session  outbound.MCPSession
	err      error
	connects int
	secrets  map[string]string
}

func (c *fakeMCPConnector) Connect(_ context.Context, _ domain.MCPServer, secrets map[string]string) (outbound.MCPSession, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connects++
	c.secrets = cloneHeaders(secrets)
	return c.session, c.err
}

type serviceFixture struct {
	service *Service
	store   *fakes.CatalogStore
	http    *fakeHTTPInvoker
	mcp     *fakeMCPConnector
	admin   domain.Principal
	member  domain.Principal
	agentID uuid.UUID
	now     time.Time
}

func newServiceFixture(t *testing.T) serviceFixture {
	t.Helper()
	store := fakes.NewCatalogStore()
	now := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	adminID, providerID, agentID := uuid.New(), uuid.New(), uuid.New()
	if err := store.CreateProvider(t.Context(), domain.Provider{ID: providerID, Name: "Provider", Revision: 1, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgent(t.Context(), domain.Agent{ID: agentID, Name: "Agent", ProviderID: providerID, MaxIterations: 8, TimeoutSeconds: 120, CreatedBy: adminID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	httpInvoker := &fakeHTTPInvoker{invocation: domain.HTTPToolInvocation{Body: "ok"}}
	mcpConnector := &fakeMCPConnector{session: &fakeMCPSession{}}
	service, err := NewService(Dependencies{Tools: store, Connections: store, MCPServers: store, Bindings: store, Agents: store, Tx: store, Cipher: fakes.Cipher{}, HTTP: httpInvoker, MCP: mcpConnector, URLs: urlPolicyFunc(func(string) error { return nil }), Clock: fakes.Clock{Time: now}, IDs: fakes.IDs{}})
	if err != nil {
		t.Fatal(err)
	}
	return serviceFixture{service: service, store: store, http: httpInvoker, mcp: mcpConnector, admin: domain.Principal{Kind: domain.PrincipalUser, UserID: adminID, Role: domain.RoleAdmin}, member: domain.Principal{Kind: domain.PrincipalUser, UserID: uuid.New(), Role: domain.RoleMember}, agentID: agentID, now: now}
}

func validToolCommand() inbound.HTTPToolCreateCommand {
	return inbound.HTTPToolCreateCommand{Slug: "weather", DisplayName: "Thời tiết", Description: "Xem thời tiết", Method: domain.HTTPToolGET, URLTemplate: "https://api.example.com/weather/{city}", Params: []domain.ToolParam{{Name: "city", Type: domain.ToolParamString, Required: true, Location: domain.ToolParamPath}}, PublicHeaders: map[string]string{"Accept": "application/json"}, SecretHeaders: map[string]string{"Authorization": "Bearer private"}, TimeoutSeconds: 15}
}

func validAPIConnectionCommand() inbound.APIConnectionCreateCommand {
	return inbound.APIConnectionCreateCommand{Slug: "orders", DisplayName: "Orders API", BaseURL: "https://api.example.com/v1", PublicHeaders: map[string]string{"X-Workspace": "shared"}, SecretHeaders: map[string]string{"Authorization": "Bearer shared"}}
}

func validMCPCommand() inbound.MCPServerCreateCommand {
	return inbound.MCPServerCreateCommand{Slug: "office", DisplayName: "Văn phòng", URL: "https://mcp.example.com", SecretHeaders: map[string]string{"Authorization": "Bearer mcp"}}
}

func TestServiceRequiresAllDependencies(t *testing.T) {
	if _, err := NewService(Dependencies{}); err == nil {
		t.Fatal("missing dependencies accepted")
	}
}

func TestHTTPToolCRUDSecretsPaginationAndTest(t *testing.T) {
	fixture := newServiceFixture(t)
	if _, err := fixture.service.CreateTool(t.Context(), fixture.member, validToolCommand()); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member create err=%v", err)
	}
	tool, err := fixture.service.CreateTool(t.Context(), fixture.admin, validToolCommand())
	if err != nil || len(tool.SecretHeadersCiphertext) == 0 || len(tool.SecretHeaderNames) != 1 {
		t.Fatalf("tool=%+v err=%v", tool, err)
	}
	if string(tool.SecretHeadersCiphertext) == "Bearer private" {
		t.Fatal("secret was stored as plaintext")
	}
	got, err := fixture.service.GetTool(t.Context(), fixture.member, tool.ID)
	if err != nil || got.ID != tool.ID {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	page, err := fixture.service.ListTools(t.Context(), fixture.member, inbound.PageRequest{})
	if err != nil || len(page.Items) != 1 || page.NextCursor != nil {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if _, err = fixture.service.ListTools(t.Context(), fixture.member, inbound.PageRequest{Cursor: "bad"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("cursor err=%v", err)
	}
	secondCommand := validToolCommand()
	secondCommand.Slug = "weather-two"
	if _, err = fixture.service.CreateTool(t.Context(), fixture.admin, secondCommand); err != nil {
		t.Fatal(err)
	}
	firstPage, err := fixture.service.ListTools(t.Context(), fixture.member, inbound.PageRequest{Limit: 1})
	if err != nil || len(firstPage.Items) != 1 || firstPage.NextCursor == nil {
		t.Fatalf("first page=%+v err=%v", firstPage, err)
	}
	secondPage, err := fixture.service.ListTools(t.Context(), fixture.member, inbound.PageRequest{Limit: 1, Cursor: *firstPage.NextCursor})
	if err != nil || len(secondPage.Items) != 1 || secondPage.NextCursor != nil || secondPage.Items[0].ID == firstPage.Items[0].ID {
		t.Fatalf("second page=%+v err=%v", secondPage, err)
	}
	name := "Thời tiết mới"
	updated, err := fixture.service.UpdateTool(t.Context(), fixture.admin, tool.ID, inbound.HTTPToolUpdateCommand{DisplayName: &name})
	if err != nil || updated.DisplayName != name || len(updated.SecretHeaderNames) != 1 {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	result, err := fixture.service.TestTool(t.Context(), fixture.admin, tool.ID, json.RawMessage(`{"city":"Huế"}`))
	if err != nil || !result.OK || fixture.http.secrets["Authorization"] != "Bearer private" {
		t.Fatalf("result=%+v secrets=%v err=%v", result, fixture.http.secrets, err)
	}
	fixture.http.invocation = domain.HTTPToolInvocation{Body: "bad", IsError: true}
	result, err = fixture.service.TestTool(t.Context(), fixture.admin, tool.ID, json.RawMessage(`{"city":"Huế"}`))
	if err != nil || result.OK || result.Failure == nil || result.Failure.Code != "tool_failed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	clearSecrets := domain.Change[map[string]string]{Set: true}
	updated, err = fixture.service.UpdateTool(t.Context(), fixture.admin, tool.ID, inbound.HTTPToolUpdateCommand{SecretHeaders: clearSecrets})
	if err != nil || len(updated.SecretHeaderNames) != 0 || len(updated.SecretHeadersCiphertext) != 0 {
		t.Fatalf("cleared=%+v err=%v", updated, err)
	}
	if err = fixture.service.DeleteTool(t.Context(), fixture.admin, tool.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.service.GetTool(t.Context(), fixture.member, tool.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted get err=%v", err)
	}
}

func TestAPIConnectionCRUDAndConnectedToolResolution(t *testing.T) {
	fixture := newServiceFixture(t)
	if _, err := fixture.service.CreateAPIConnection(t.Context(), fixture.member, validAPIConnectionCommand()); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("member create err=%v", err)
	}
	connection, err := fixture.service.CreateAPIConnection(t.Context(), fixture.admin, validAPIConnectionCommand())
	if err != nil || len(connection.SecretHeadersCiphertext) == 0 || len(connection.SecretHeaderNames) != 1 {
		t.Fatalf("connection=%+v err=%v", connection, err)
	}
	renamed := "Orders API renamed"
	connection, err = fixture.service.UpdateAPIConnection(t.Context(), fixture.admin, connection.ID, inbound.APIConnectionUpdateCommand{DisplayName: &renamed})
	if err != nil || len(connection.SecretHeaderNames) != 1 {
		t.Fatalf("omitted secret update=%+v err=%v", connection, err)
	}
	page, err := fixture.service.ListAPIConnections(t.Context(), fixture.member, inbound.PageRequest{})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != connection.ID {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	command := validToolCommand()
	command.ConnectionID = &connection.ID
	command.URLTemplate = "/weather/{city}"
	command.PublicHeaders = map[string]string{"X-Operation": "weather"}
	command.SecretHeaders = map[string]string{"X-Operation-Token": "private"}
	tool, err := fixture.service.CreateTool(t.Context(), fixture.admin, command)
	if err != nil || tool.ConnectionID == nil || *tool.ConnectionID != connection.ID {
		t.Fatalf("tool=%+v err=%v", tool, err)
	}
	secondCommand := command
	secondCommand.Slug = "forecast"
	secondCommand.DisplayName = "Dự báo"
	secondTool, err := fixture.service.CreateTool(t.Context(), fixture.admin, secondCommand)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.service.TestTool(t.Context(), fixture.admin, tool.ID, json.RawMessage(`{"city":"Huế"}`))
	if err != nil || !result.OK || fixture.http.tool.URLTemplate != "https://api.example.com/v1/weather/{city}" || fixture.http.tool.PublicHeaders["X-Workspace"] != "shared" || fixture.http.tool.PublicHeaders["X-Operation"] != "weather" || fixture.http.secrets["Authorization"] != "Bearer shared" || fixture.http.secrets["X-Operation-Token"] != "private" {
		t.Fatalf("result=%+v tool=%+v headers=%v secrets=%v err=%v", result, fixture.http.tool, fixture.http.tool.PublicHeaders, fixture.http.secrets, err)
	}
	if err = fixture.service.DeleteAPIConnection(t.Context(), fixture.admin, connection.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("referenced connection delete err=%v", err)
	}
	if _, err = fixture.service.ReplaceAgentTools(t.Context(), fixture.admin, fixture.agentID, domain.AgentToolBindings{ToolIDs: []uuid.UUID{tool.ID, secondTool.ID}}); err != nil {
		t.Fatal(err)
	}
	cipher := &countingCipher{}
	fixture.service.Cipher = cipher
	cipher.reset()
	beforeRotation, err := fixture.service.Resolve(t.Context(), fixture.agentID)
	if err != nil || cipher.countPrefix("api_connection:") != 1 {
		t.Fatalf("snapshot resolve err=%v connection decrypts=%d", err, cipher.countPrefix("api_connection:"))
	}
	newBase := "https://api-two.example.com/v2"
	replacement := map[string]string{"Authorization": "Bearer rotated"}
	updated, err := fixture.service.UpdateAPIConnection(t.Context(), fixture.admin, connection.ID, inbound.APIConnectionUpdateCommand{BaseURL: &newBase, SecretHeaders: domain.Change[map[string]string]{Set: true, Value: &replacement}})
	if err != nil || updated.BaseURL != newBase {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if _, err = fixture.service.TestTool(t.Context(), fixture.admin, tool.ID, json.RawMessage(`{"city":"Huế"}`)); err != nil || fixture.http.tool.URLTemplate != "https://api-two.example.com/v2/weather/{city}" || fixture.http.secrets["Authorization"] != "Bearer rotated" {
		t.Fatalf("rotated tool=%+v secrets=%v err=%v", fixture.http.tool, fixture.http.secrets, err)
	}
	cipher.reset()
	afterRotation, err := fixture.service.Resolve(t.Context(), fixture.agentID)
	if err != nil || cipher.countPrefix("api_connection:") != 1 {
		t.Fatalf("rotated snapshot err=%v connection decrypts=%d", err, cipher.countPrefix("api_connection:"))
	}
	if result := beforeRotation.Execute(t.Context(), domain.ToolCall{ID: "before", Name: "http_weather", Arguments: json.RawMessage(`{"city":"Huế"}`)}); result.IsError || fixture.http.tool.URLTemplate != "https://api.example.com/v1/weather/{city}" || fixture.http.secrets["Authorization"] != "Bearer shared" {
		t.Fatalf("old snapshot result=%+v tool=%+v secrets=%v", result, fixture.http.tool, fixture.http.secrets)
	}
	if result := afterRotation.Execute(t.Context(), domain.ToolCall{ID: "after", Name: "http_weather", Arguments: json.RawMessage(`{"city":"Huế"}`)}); result.IsError || fixture.http.tool.URLTemplate != "https://api-two.example.com/v2/weather/{city}" || fixture.http.secrets["Authorization"] != "Bearer rotated" {
		t.Fatalf("new snapshot result=%+v tool=%+v secrets=%v", result, fixture.http.tool, fixture.http.secrets)
	}
	cleared, err := fixture.service.UpdateAPIConnection(t.Context(), fixture.admin, connection.ID, inbound.APIConnectionUpdateCommand{SecretHeaders: domain.Change[map[string]string]{Set: true}})
	if err != nil || len(cleared.SecretHeaderNames) != 0 || len(cleared.SecretHeadersCiphertext) != 0 {
		t.Fatalf("null clear=%+v err=%v", cleared, err)
	}
	if _, err = fixture.service.UpdateAPIConnection(t.Context(), fixture.admin, connection.ID, inbound.APIConnectionUpdateCommand{SecretHeaders: domain.Change[map[string]string]{Set: true, Value: &replacement}}); err != nil {
		t.Fatal(err)
	}
	emptySecrets := map[string]string{}
	cleared, err = fixture.service.UpdateAPIConnection(t.Context(), fixture.admin, connection.ID, inbound.APIConnectionUpdateCommand{SecretHeaders: domain.Change[map[string]string]{Set: true, Value: &emptySecrets}})
	if err != nil || len(cleared.SecretHeaderNames) != 0 || len(cleared.SecretHeadersCiphertext) != 0 {
		t.Fatalf("empty object clear=%+v err=%v", cleared, err)
	}
	if err = fixture.service.DeleteTool(t.Context(), fixture.admin, tool.ID); err != nil {
		t.Fatal(err)
	}
	if err = fixture.service.DeleteTool(t.Context(), fixture.admin, secondTool.ID); err != nil {
		t.Fatal(err)
	}
	if err = fixture.service.DeleteAPIConnection(t.Context(), fixture.admin, connection.ID); err != nil {
		t.Fatal(err)
	}
}

func TestMCPServerCRUDRefreshAndFailure(t *testing.T) {
	fixture := newServiceFixture(t)
	server, err := fixture.service.CreateMCPServer(t.Context(), fixture.admin, validMCPCommand())
	if err != nil || server.AllowedTools != nil || len(server.SecretHeaderNames) != 1 {
		t.Fatalf("server=%+v err=%v", server, err)
	}
	session := &fakeMCPSession{tools: []domain.MCPTool{{Name: "search", Description: "Tìm", InputSchema: json.RawMessage(`{"type":"object"}`)}, {Name: "read", Description: "Đọc", InputSchema: json.RawMessage(`{"type":"object"}`)}}}
	fixture.mcp.session = session
	refreshed, err := fixture.service.RefreshMCPServer(t.Context(), fixture.admin, server.ID)
	if err != nil || refreshed.Status != domain.ConnectionOK || len(refreshed.Tools) != 2 || fixture.mcp.secrets["Authorization"] != "Bearer mcp" || !session.closed {
		t.Fatalf("refreshed=%+v err=%v", refreshed, err)
	}
	empty := []string{}
	updated, err := fixture.service.UpdateMCPServer(t.Context(), fixture.admin, server.ID, inbound.MCPServerUpdateCommand{AllowedTools: domain.Change[[]string]{Set: true, Value: &empty}})
	if err != nil || updated.AllowedTools == nil || len(*updated.AllowedTools) != 0 || len(updated.Tools) != 2 {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	url := "https://new-mcp.example.com"
	updated, err = fixture.service.UpdateMCPServer(t.Context(), fixture.admin, server.ID, inbound.MCPServerUpdateCommand{URL: &url})
	if err != nil || updated.Status != domain.ConnectionUnchecked || len(updated.Tools) != 0 || updated.LastSyncedAt != nil {
		t.Fatalf("endpoint update=%+v err=%v", updated, err)
	}
	fixture.mcp.err = context.DeadlineExceeded
	failed, err := fixture.service.RefreshMCPServer(t.Context(), fixture.admin, server.ID)
	if err != nil || failed.Status != domain.ConnectionFailing || failed.LastError == nil || failed.LastError.Code != "tool_failed" {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	page, err := fixture.service.ListMCPServers(t.Context(), fixture.member, inbound.PageRequest{})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if _, err = fixture.service.GetMCPServer(t.Context(), fixture.member, server.ID); err != nil {
		t.Fatal(err)
	}
	if err = fixture.service.DeleteMCPServer(t.Context(), fixture.admin, server.ID); err != nil {
		t.Fatal(err)
	}
}

func TestBindingsResolverLazyMCPAndRollback(t *testing.T) {
	fixture := newServiceFixture(t)
	tool, err := fixture.service.CreateTool(t.Context(), fixture.admin, validToolCommand())
	if err != nil {
		t.Fatal(err)
	}
	allowed := []string{"search"}
	command := validMCPCommand()
	command.AllowedTools = &allowed
	server, err := fixture.service.CreateMCPServer(t.Context(), fixture.admin, command)
	if err != nil {
		t.Fatal(err)
	}
	refreshSession := &fakeMCPSession{tools: []domain.MCPTool{{Name: "search", Description: "Tìm", InputSchema: json.RawMessage(`{"type":"object"}`)}, {Name: "hidden", Description: "Ẩn", InputSchema: json.RawMessage(`{"type":"object"}`)}}}
	fixture.mcp.session = refreshSession
	if _, err = fixture.service.RefreshMCPServer(t.Context(), fixture.admin, server.ID); err != nil {
		t.Fatal(err)
	}
	bindings := domain.AgentToolBindings{ToolIDs: []uuid.UUID{tool.ID}, MCPServerIDs: []uuid.UUID{server.ID}}
	saved, err := fixture.service.ReplaceAgentTools(t.Context(), fixture.admin, fixture.agentID, bindings)
	if err != nil || len(saved.ToolIDs) != 1 || len(saved.MCPServerIDs) != 1 {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
	if _, err = fixture.service.ReplaceAgentTools(t.Context(), fixture.admin, fixture.agentID, domain.AgentToolBindings{ToolIDs: []uuid.UUID{uuid.New()}}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing binding err=%v", err)
	}
	if _, err = fixture.service.ReplaceAgentTools(t.Context(), fixture.admin, fixture.agentID, domain.AgentToolBindings{ToolIDs: []uuid.UUID{tool.ID, tool.ID}}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("duplicate binding err=%v", err)
	}
	unchanged, _ := fixture.service.GetAgentTools(t.Context(), fixture.member, fixture.agentID)
	if len(unchanged.ToolIDs) != 1 || unchanged.ToolIDs[0] != tool.ID {
		t.Fatalf("rollback lost binding: %+v", unchanged)
	}
	runSession := &fakeMCPSession{result: domain.ToolResult{Content: "mcp ok"}}
	fixture.mcp.session = runSession
	fixture.mcp.connects = 0
	set, err := fixture.service.Resolve(t.Context(), fixture.agentID)
	if err != nil || len(set.Specs()) != 2 {
		t.Fatalf("specs=%+v err=%v", set.Specs(), err)
	}
	fixture.http.invocation.Truncated = true
	var httpName, mcpName string
	for _, spec := range set.Specs() {
		if spec.Name == "http_weather" {
			httpName = spec.Name
		} else {
			mcpName = spec.Name
		}
	}
	if result := set.Execute(t.Context(), domain.ToolCall{ID: "http-call", Name: httpName, Arguments: json.RawMessage(`{"city":"Huế"}`)}); result.IsError || result.Content != "ok" || !result.Truncated {
		t.Fatalf("HTTP result=%+v", result)
	}
	for range 2 {
		if result := set.Execute(t.Context(), domain.ToolCall{ID: "mcp-call", Name: mcpName, Arguments: json.RawMessage(`{}`)}); result.IsError || result.Content != "mcp ok" {
			t.Fatalf("MCP result=%+v", result)
		}
	}
	if fixture.mcp.connects != 1 || len(runSession.calls) != 2 {
		t.Fatalf("connects=%d calls=%v", fixture.mcp.connects, runSession.calls)
	}
	unknown := set.Execute(t.Context(), domain.ToolCall{ID: "unknown", Name: "missing", Arguments: json.RawMessage(`{}`)})
	if !unknown.IsError {
		t.Fatal("unknown tool succeeded")
	}
	if err = set.Close(); err != nil || !runSession.closed || set.Close() != nil {
		t.Fatalf("close err=%v closed=%v", err, runSession.closed)
	}
}

func TestValidationAndSafeFailureBranches(t *testing.T) {
	fixture := newServiceFixture(t)
	fixture.service.URLs = urlPolicyFunc(func(string) error { return domain.ErrEgressDenied })
	if _, err := fixture.service.CreateTool(t.Context(), fixture.admin, validToolCommand()); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("tool URL err=%v", err)
	}
	if _, err := fixture.service.CreateMCPServer(t.Context(), fixture.admin, validMCPCommand()); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("MCP URL err=%v", err)
	}
	for _, test := range []struct {
		err     error
		message string
	}{
		{context.DeadlineExceeded, "Công cụ hết thời gian phản hồi."},
		{domain.ErrEgressDenied, "Địa chỉ công cụ bị chính sách mạng chặn. Kiểm tra lại địa chỉ hoặc cấu hình mạng."},
		{domain.ErrProviderUnreachable, "Không kết nối được tới hệ thống đích. Kiểm tra dịch vụ đích đang chạy và địa chỉ đúng."},
		{domain.ErrValidation, "Đối số hoặc cấu hình công cụ không hợp lệ."},
		{errors.New("private upstream detail"), "Không thể chạy công cụ."},
	} {
		if failure := toolFailure(test.err); failure.Code != "tool_failed" || failure.Message != test.message {
			t.Fatalf("failure=%+v", failure)
		}
	}
	if _, err := pageOptions(inbound.PageRequest{Limit: 101}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("limit err=%v", err)
	}
}

func TestMergeHeadersUsesCaseInsensitiveOverride(t *testing.T) {
	merged := mergeHeaders(map[string]string{"Authorization": "shared", "X-Base": "base"}, map[string]string{"authorization": "operation"})
	if len(merged) != 2 || merged["authorization"] != "operation" || merged["Authorization"] != "" || merged["X-Base"] != "base" {
		t.Fatalf("merged=%v", merged)
	}
}
