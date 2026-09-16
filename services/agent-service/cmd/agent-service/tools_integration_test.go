//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestToolingHTTPAPIAndRunEngine(t *testing.T) {
	server, _ := identityTestServer(t)
	catalog := newCatalogHTTP(t, server)
	var toolCalls atomic.Int32
	toolServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		toolCalls.Add(1)
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer tool-private-value" {
			t.Errorf("tool request method=%s auth=%q", request.Method, request.Header.Get("Authorization"))
		}
		var body map[string]any
		if json.NewDecoder(request.Body).Decode(&body) != nil || body["value"] != "hello" {
			t.Errorf("tool body=%v", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"answer":"from tool"}`)
	}))
	defer toolServer.Close()

	var llmCalls atomic.Int32
	llmServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Errorf("LLM path=%s", request.URL.Path)
		}
		var body map[string]any
		if json.NewDecoder(request.Body).Decode(&body) != nil {
			t.Error("decode LLM request")
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		if llmCalls.Add(1) == 1 {
			tools, _ := body["tools"].([]any)
			if len(tools) != 1 {
				t.Errorf("LLM tools=%v", body["tools"])
			}
			_, _ = fmt.Fprint(writer, "data: {\"id\":\"one\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"http_echo\",\"arguments\":\"{\\\"value\\\":\\\"hello\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		messages, _ := body["messages"].([]any)
		encoded, _ := json.Marshal(messages)
		if !strings.Contains(string(encoded), "from tool") || !strings.Contains(string(encoded), "call-1") {
			t.Errorf("tool result not replayed: %s", encoded)
		}
		_, _ = fmt.Fprint(writer, "data: {\"id\":\"two\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Đã dùng công cụ\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer llmServer.Close()

	provider := catalog.provider("openai_compatible", llmServer.URL+"/v1", "Tool connection")
	var agent gen.Agent
	catalog.request("POST", "/v1/agents", map[string]any{"name": "Tool assistant", "provider_id": provider.Id, "model": "test", "max_output_tokens": 32}, 201, &agent)
	var connection gen.ApiConnection
	catalog.request("POST", "/v1/api-connections", map[string]any{"slug": "tool-api", "display_name": "Tool API", "base_url": toolServer.URL, "secret_headers": map[string]string{"Authorization": "Bearer tool-private-value"}}, 201, &connection)
	if len(connection.SecretHeaderNames) != 1 || connection.SecretHeaderNames[0] != "Authorization" {
		t.Fatalf("connection response=%+v", connection)
	}
	var tool gen.HttpTool
	catalog.request("POST", "/v1/tools", map[string]any{"slug": "echo", "display_name": "Echo", "description": "Echo value", "method": "POST", "connection_id": connection.Id, "url_template": "/", "params": []map[string]any{{"name": "value", "type": "string", "description": "Value", "required": true, "in": "body"}}}, 201, &tool)
	if tool.ConnectionId.MustGet() != connection.Id || len(tool.SecretHeaderNames) != 0 {
		t.Fatalf("tool response=%+v", tool)
	}
	var tested gen.ToolTestResult
	catalog.request("POST", "/v1/tools/"+tool.Id.String()+"/test", map[string]any{"args": map[string]string{"value": "hello"}}, 200, &tested)
	if !tested.Ok || tested.Body != `{"answer":"from tool"}` {
		t.Fatalf("test result=%+v", tested)
	}
	var bindings gen.AgentToolBindings
	catalog.request("PUT", "/v1/agents/"+agent.Id.String()+"/tools", map[string]any{"tool_ids": []string{tool.Id.String()}, "mcp_server_ids": []string{}}, 200, &bindings)
	if len(bindings.ToolIds) != 1 || bindings.ToolIds[0] != tool.Id {
		t.Fatalf("bindings=%+v", bindings)
	}
	var run gen.Run
	catalog.request("POST", "/v1/agents/"+agent.Id.String()+"/runs", map[string]any{"input": map[string]string{"message": "Dùng công cụ"}, "mode": "sync"}, 200, &run)
	if run.Status != gen.RunStatusSucceeded || run.Output.MustGet() != "Đã dùng công cụ" || run.Iterations != 2 || toolCalls.Load() != 2 || llmCalls.Load() != 2 {
		t.Fatalf("run=%+v tool_calls=%d llm_calls=%d", run, toolCalls.Load(), llmCalls.Load())
	}
	var spans gen.SpanPage
	catalog.request("GET", "/v1/runs/"+run.Id.String()+"/spans", nil, 200, &spans)
	foundToolSpan := false
	for _, span := range spans.Items {
		if span.Kind == gen.SpanKindToolCall && span.ToolName.MustGet() == "http_echo" {
			foundToolSpan = true
		}
	}
	if !foundToolSpan {
		t.Fatalf("spans=%+v", spans.Items)
	}
	var patched gen.ApiConnection
	catalog.request("PATCH", "/v1/api-connections/"+connection.Id.String(), map[string]any{"display_name": "Tool API renamed"}, 200, &patched)
	if len(patched.SecretHeaderNames) != 1 {
		t.Fatalf("omitted secrets were not preserved: %+v", patched)
	}
	catalog.request("PATCH", "/v1/api-connections/"+connection.Id.String(), map[string]any{"secret_headers": nil}, 200, &patched)
	if patched.SecretHeaderNames == nil || len(patched.SecretHeaderNames) != 0 {
		t.Fatalf("null secret clear response=%+v", patched)
	}
	catalog.request("PATCH", "/v1/api-connections/"+connection.Id.String(), map[string]any{"secret_headers": map[string]string{"Authorization": "Bearer replacement"}}, 200, &patched)
	catalog.request("PATCH", "/v1/api-connections/"+connection.Id.String(), map[string]any{"secret_headers": map[string]string{}}, 200, &patched)
	if patched.SecretHeaderNames == nil || len(patched.SecretHeaderNames) != 0 {
		t.Fatalf("empty secret clear response=%+v", patched)
	}
	catalog.request("DELETE", "/v1/api-connections/"+connection.Id.String(), nil, 409, nil)
}

type integrationGreetInput struct {
	Name string `json:"name"`
}

type integrationGreetOutput struct {
	Message string `json:"message"`
}

func TestMCPAPIRefreshAndRunEngine(t *testing.T) {
	server, _ := identityTestServer(t)
	catalog := newCatalogHTTP(t, server)
	var mcpCalls atomic.Int32
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "integration", Version: "1.0.0"}, nil)
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "greet", Description: "Chào người dùng"}, func(_ context.Context, _ *mcp.CallToolRequest, input integrationGreetInput) (*mcp.CallToolResult, integrationGreetOutput, error) {
		mcpCalls.Add(1)
		return nil, integrationGreetOutput{Message: "Xin chào " + input.Name}, nil
	})
	mcpHTTP := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		if request.Header.Get("Authorization") != "Bearer mcp-private-value" {
			t.Errorf("MCP auth=%q", request.Header.Get("Authorization"))
			return nil
		}
		return mcpServer
	}, nil))
	defer mcpHTTP.Close()

	var llmCalls atomic.Int32
	llmServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]any
		if json.NewDecoder(request.Body).Decode(&body) != nil {
			t.Error("decode LLM request")
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		if llmCalls.Add(1) == 1 {
			encoded, _ := json.Marshal(body["tools"])
			if !strings.Contains(string(encoded), "mcp_office_greet") {
				t.Errorf("MCP tool not declared: %s", encoded)
			}
			_, _ = fmt.Fprint(writer, "data: {\"id\":\"one\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-mcp\",\"type\":\"function\",\"function\":{\"name\":\"mcp_office_greet\",\"arguments\":\"{\\\"name\\\":\\\"Huế\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		messages, _ := json.Marshal(body["messages"])
		if !strings.Contains(string(messages), "Xin chào Huế") {
			t.Errorf("MCP result not replayed: %s", messages)
		}
		_, _ = fmt.Fprint(writer, "data: {\"id\":\"two\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"MCP hoạt động\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer llmServer.Close()

	provider := catalog.provider("openai_compatible", llmServer.URL+"/v1", "MCP connection")
	var agent gen.Agent
	catalog.request("POST", "/v1/agents", map[string]any{"name": "MCP assistant", "provider_id": provider.Id, "model": "test", "max_output_tokens": 32}, 201, &agent)
	var saved gen.McpServer
	catalog.request("POST", "/v1/mcp-servers", map[string]any{"slug": "office", "display_name": "Office", "url": mcpHTTP.URL, "secret_headers": map[string]string{"Authorization": "Bearer mcp-private-value"}}, 201, &saved)
	if len(saved.Tools) != 0 || saved.Status != gen.ConnectionStatusUnchecked {
		t.Fatalf("saved MCP=%+v", saved)
	}
	catalog.request("POST", "/v1/mcp-servers/"+saved.Id.String()+"/refresh", nil, 200, &saved)
	if saved.Status != gen.ConnectionStatusOk || len(saved.Tools) != 1 || saved.Tools[0].Name != "greet" {
		t.Fatalf("refreshed MCP=%+v", saved)
	}
	catalog.request("PUT", "/v1/agents/"+agent.Id.String()+"/tools", map[string]any{"tool_ids": []string{}, "mcp_server_ids": []string{saved.Id.String()}}, 200, nil)
	var run gen.Run
	catalog.request("POST", "/v1/agents/"+agent.Id.String()+"/runs", map[string]any{"input": map[string]string{"message": "Chào Huế"}, "mode": "sync"}, 200, &run)
	if run.Status != gen.RunStatusSucceeded || run.Output.MustGet() != "MCP hoạt động" || mcpCalls.Load() != 1 || llmCalls.Load() != 2 {
		t.Fatalf("run=%+v mcp_calls=%d llm_calls=%d", run, mcpCalls.Load(), llmCalls.Load())
	}
}
