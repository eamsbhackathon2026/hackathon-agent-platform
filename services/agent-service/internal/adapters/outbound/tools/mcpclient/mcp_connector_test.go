package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/platform/netguard"
)

type greetInput struct {
	Name string `json:"name" jsonschema:"Tên người nhận"`
}

type greetOutput struct {
	Message string `json:"message"`
}

func TestConnectorListsPagedToolsAndCallsThem(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, &mcp.ServerOptions{PageSize: 1})
	mcp.AddTool(server, &mcp.Tool{Name: "greet", Description: "Chào người dùng"}, func(_ context.Context, _ *mcp.CallToolRequest, input greetInput) (*mcp.CallToolResult, greetOutput, error) {
		return nil, greetOutput{Message: "Xin chào " + input.Name}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "fail", Description: "Trả lỗi"}, func(context.Context, *mcp.CallToolRequest, greetInput) (*mcp.CallToolResult, greetOutput, error) {
		return nil, greetOutput{}, errors.New("expected tool error")
	})
	var authorized atomic.Bool
	handler := mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		if request.Header.Get("Authorization") == "Bearer secret" {
			authorized.Store(true)
			return server
		}
		return nil
	}, nil)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	guard, _ := netguard.New(netguard.Config{Development: true})
	connector, err := New(guard.NewHTTPClient())
	if err != nil {
		t.Fatal(err)
	}
	session, err := connector.Connect(t.Context(), domain.MCPServer{URL: httpServer.URL}, map[string]string{"Authorization": "Bearer secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := session.Close(); closeErr != nil {
			t.Errorf("close session: %v", closeErr)
		}
	}()
	tools, err := session.ListTools(t.Context())
	if err != nil || len(tools) != 2 || tools[0].Name == tools[1].Name {
		t.Fatalf("tools=%+v err=%v", tools, err)
	}
	for _, tool := range tools {
		if !json.Valid(tool.InputSchema) {
			t.Fatalf("invalid schema for %s: %s", tool.Name, tool.InputSchema)
		}
	}
	result, err := session.CallTool(t.Context(), "greet", json.RawMessage(`{"name":"Huế"}`))
	if err != nil || result.IsError || !strings.Contains(result.Content, "Xin chào Huế") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	failed, err := session.CallTool(t.Context(), "fail", json.RawMessage(`{"name":"Huế"}`))
	if err != nil || !failed.IsError || failed.Content == "" {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	if !authorized.Load() {
		t.Fatal("secret header was not attached")
	}
}

func TestConnectorRejectsInvalidArgumentsBeforeCall(t *testing.T) {
	session := &sessionAdapter{}
	if _, err := session.CallTool(t.Context(), "unused", json.RawMessage(`[]`)); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("err=%v", err)
	}
}

func TestConnectorReportsUnavailableServer(t *testing.T) {
	httpServer := httptest.NewServer(http.NotFoundHandler())
	endpoint := httpServer.URL
	httpServer.Close()
	guard, _ := netguard.New(netguard.Config{Development: true})
	connector, _ := New(guard.NewHTTPClient())
	if _, err := connector.Connect(t.Context(), domain.MCPServer{URL: endpoint}, nil); err == nil {
		t.Fatal("unavailable MCP server connected successfully")
	}
}

func TestConnectorRejectsOversizedCatalogMetadata(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "oversized", Description: strings.Repeat("a", 4001)}, func(context.Context, *mcp.CallToolRequest, greetInput) (*mcp.CallToolResult, greetOutput, error) {
		return nil, greetOutput{}, nil
	})
	httpServer := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	defer httpServer.Close()
	guard, _ := netguard.New(netguard.Config{Development: true})
	connector, _ := New(guard.NewHTTPClient())
	session, err := connector.Connect(t.Context(), domain.MCPServer{URL: httpServer.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	if _, err = session.ListTools(t.Context()); !errors.Is(err, domain.ErrProviderBadRequest) {
		t.Fatalf("oversized metadata err=%v", err)
	}
}
