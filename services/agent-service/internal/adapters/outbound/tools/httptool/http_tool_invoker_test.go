package httptool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/platform/netguard"
)

func TestInvokerBuildsGuardedRequestAndTruncatesText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.EscapedPath() != "/items/a%2Fb" || request.URL.Query().Get("q") != "weather" {
			t.Errorf("request=%s %s?%s", request.Method, request.URL.EscapedPath(), request.URL.RawQuery)
		}
		if request.Header.Get("X-Public") != "public" || request.Header.Get("Authorization") != "Bearer private" || request.Header.Get("Content-Type") != "application/json" {
			t.Errorf("headers=%v", request.Header)
		}
		body, _ := io.ReadAll(request.Body)
		var value map[string]any
		if json.Unmarshal(body, &value) != nil || value["message"] != "xin chào" {
			t.Errorf("body=%s", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, strings.Repeat("a", maxResponseBytes+20))
	}))
	defer server.Close()
	guard, err := netguard.New(netguard.Config{Development: true})
	if err != nil {
		t.Fatal(err)
	}
	invoker, err := New(guard.NewHTTPClient())
	if err != nil {
		t.Fatal(err)
	}
	tool := domain.HTTPTool{Method: domain.HTTPToolPOST, URLTemplate: server.URL + "/items/{id}", Params: []domain.ToolParam{{Name: "id", Type: domain.ToolParamString, Required: true, Location: domain.ToolParamPath}, {Name: "q", Type: domain.ToolParamString, Location: domain.ToolParamQuery}, {Name: "message", Type: domain.ToolParamString, Location: domain.ToolParamBody}}, PublicHeaders: map[string]string{"X-Public": "public"}, TimeoutSeconds: 15}
	result, err := invoker.Invoke(t.Context(), tool, map[string]string{"Authorization": "Bearer private"}, json.RawMessage(`{"id":"a/b","q":"weather","message":"xin chào"}`))
	if err != nil || result.StatusCode == nil || *result.StatusCode != 200 || result.IsError || !result.Truncated || len(result.Body) != maxResponseBytes {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestInvokerHandlesGETErrorsBinaryAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/binary":
			writer.Header().Set("Content-Type", "application/octet-stream")
			_, _ = writer.Write([]byte{0, 1, 2})
		case "/error":
			writer.Header().Set("Content-Type", "text/plain")
			writer.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(writer, "upstream failed")
		case "/slow":
			<-request.Context().Done()
		default:
			t.Errorf("unexpected path %s", request.URL.Path)
		}
	}))
	defer server.Close()
	guard, _ := netguard.New(netguard.Config{Development: true})
	invoker, _ := New(guard.NewHTTPClient())
	for _, test := range []struct {
		path, content string
		isError       bool
	}{{"/binary", "[nội dung nhị phân bị bỏ qua]", false}, {"/error", "upstream failed", true}} {
		tool := domain.HTTPTool{Method: domain.HTTPToolGET, URLTemplate: server.URL + test.path, TimeoutSeconds: 15}
		result, err := invoker.Invoke(t.Context(), tool, nil, json.RawMessage(`{}`))
		if err != nil || result.Body != test.content || result.IsError != test.isError {
			t.Fatalf("path=%s result=%+v err=%v", test.path, result, err)
		}
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	tool := domain.HTTPTool{Method: domain.HTTPToolGET, URLTemplate: server.URL + "/error", TimeoutSeconds: 15}
	if _, err := invoker.Invoke(cancelled, tool, nil, json.RawMessage(`{}`)); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	slow := domain.HTTPTool{Method: domain.HTTPToolGET, URLTemplate: server.URL + "/slow", TimeoutSeconds: 1}
	started := time.Now()
	if _, err := invoker.Invoke(t.Context(), slow, nil, json.RawMessage(`{}`)); !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 1500*time.Millisecond {
		t.Fatalf("timeout err=%v duration=%s", err, time.Since(started))
	}
}

func TestInvokerRejectsBadArgumentsAndMetadataAddress(t *testing.T) {
	guard, _ := netguard.New(netguard.Config{Development: true})
	invoker, _ := New(guard.NewHTTPClient())
	tool := domain.HTTPTool{Method: domain.HTTPToolGET, URLTemplate: "http://169.254.169.254/latest", Params: []domain.ToolParam{{Name: "id", Type: domain.ToolParamInteger, Required: true, Location: domain.ToolParamQuery}}, TimeoutSeconds: 15}
	if _, err := invoker.Invoke(t.Context(), tool, nil, json.RawMessage(`{"id":"bad"}`)); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("argument err=%v", err)
	}
	if _, err := invoker.Invoke(t.Context(), tool, nil, json.RawMessage(`{"id":1}`)); !errors.Is(err, domain.ErrEgressDenied) {
		t.Fatalf("egress err=%v", err)
	}
}

func TestInvokerSendsStructuredBodyUnchanged(t *testing.T) {
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		received = string(body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"ok":true}`)
	}))
	defer server.Close()
	guard, err := netguard.New(netguard.Config{Development: true})
	if err != nil {
		t.Fatal(err)
	}
	invoker, err := New(guard.NewHTTPClient())
	if err != nil {
		t.Fatal(err)
	}
	tool := domain.HTTPTool{
		Method:      domain.HTTPToolPOST,
		URLTemplate: server.URL + "/transfer/precheck",
		Params: []domain.ToolParam{
			{Name: "customer_id", Type: domain.ToolParamInteger, Required: true, Location: domain.ToolParamBody},
			{Name: "session_flags", Type: domain.ToolParamObject, Location: domain.ToolParamBody},
			{Name: "recent_events", Type: domain.ToolParamArray, Location: domain.ToolParamBody},
		},
		TimeoutSeconds: 15,
	}
	arguments := `{"customer_id":42,"session_flags":{"screen_sharing":true,"remote_app":false},"recent_events":["NEW_DEVICE_LOGIN","LIMIT_INCREASE"]}`
	result, err := invoker.Invoke(t.Context(), tool, nil, json.RawMessage(arguments))
	if err != nil || result.StatusCode == nil || *result.StatusCode != 200 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var sent map[string]any
	if err = json.Unmarshal([]byte(received), &sent); err != nil {
		t.Fatalf("thân yêu cầu không phải JSON: %s", received)
	}
	flags, ok := sent["session_flags"].(map[string]any)
	if !ok || flags["screen_sharing"] != true || flags["remote_app"] != false {
		t.Fatalf("session_flags bị biến dạng: %s", received)
	}
	events, ok := sent["recent_events"].([]any)
	if !ok || len(events) != 2 || events[0] != "NEW_DEVICE_LOGIN" {
		t.Fatalf("recent_events bị biến dạng: %s", received)
	}
	// Numbers must survive the json.Number round trip without turning into strings.
	if fmt.Sprint(sent["customer_id"]) != "42" {
		t.Fatalf("customer_id bị biến dạng: %s", received)
	}
}
