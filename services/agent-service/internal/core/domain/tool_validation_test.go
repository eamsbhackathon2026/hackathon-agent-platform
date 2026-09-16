package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func validHTTPTool() HTTPTool {
	return HTTPTool{Slug: "lookup", DisplayName: "Tra cứu", Description: "Tra cứu dữ liệu", Method: HTTPToolPOST, URLTemplate: "https://api.example.com/items/{id}", Params: []ToolParam{{Name: "id", Type: ToolParamString, Required: true, Location: ToolParamPath}, {Name: "q", Type: ToolParamString, Description: "Từ khóa", Required: true, Location: ToolParamBody}}, PublicHeaders: map[string]string{"Accept": "application/json"}, TimeoutSeconds: 15}
}

func TestValidateAPIConnectionAndOperationPath(t *testing.T) {
	connection := APIConnection{Slug: "orders", DisplayName: "Orders API", BaseURL: "https://api.example.com/v1", PublicHeaders: map[string]string{"Accept": "application/json"}}
	if err := ValidateAPIConnection(connection, map[string]string{"Authorization": "Bearer secret"}); err != nil {
		t.Fatal(err)
	}
	resolved, err := JoinAPIConnectionURL(connection.BaseURL, "/orders/{order_id}")
	if err != nil || resolved != "https://api.example.com/v1/orders/{order_id}" {
		t.Fatalf("resolved=%q err=%v", resolved, err)
	}
	connectionID := uuid.New()
	tool := validHTTPTool()
	tool.ConnectionID = &connectionID
	tool.URLTemplate = "/items/{id}"
	if err = ValidateHTTPTool(tool, nil); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"//evil.example/items/{id}", "https://evil.example/items/{id}", "/../items/{id}", "/%2e%2e/items/{id}", "/%252e%252e/items/{id}", "/%2F%2Fevil/items/{id}", "/items/{id}?admin=true", "/items/{id}#fragment"} {
		tool.URLTemplate = invalid
		if err = ValidateHTTPTool(tool, nil); !errors.Is(err, ErrValidation) {
			t.Fatalf("accepted operation path %q: %v", invalid, err)
		}
	}
	for _, invalidBase := range []string{"https://user:pass@example.com", "https://api.example.com?v=1", "ftp://api.example.com", "/relative"} {
		connection.BaseURL = invalidBase
		if err = ValidateAPIConnection(connection, nil); !errors.Is(err, ErrValidation) {
			t.Fatalf("accepted base URL %q: %v", invalidBase, err)
		}
	}
}

func TestValidateHTTPToolAndGeneratedSchema(t *testing.T) {
	tool := validHTTPTool()
	if err := ValidateHTTPTool(tool, map[string]string{"Authorization": "Bearer secret"}); err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(ToolJSONSchema(tool.Params), &schema); err != nil {
		t.Fatal(err)
	}
	required, ok := schema["required"].([]any)
	if !ok || len(required) != 2 || schema["additionalProperties"] != false {
		t.Fatalf("schema=%#v", schema)
	}
	properties := schema["properties"].(map[string]any)
	if properties["q"].(map[string]any)["type"] != "string" {
		t.Fatalf("properties=%#v", properties)
	}
}

func TestValidateHTTPToolRejectsBrokenPathsAndHeaders(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*HTTPTool)
		secrets map[string]string
	}{
		{name: "undeclared placeholder", mutate: func(tool *HTTPTool) { tool.URLTemplate += "/{missing}" }},
		{name: "unused path parameter", mutate: func(tool *HTTPTool) { tool.URLTemplate = "https://api.example.com/items" }},
		{name: "optional path parameter", mutate: func(tool *HTTPTool) { tool.Params[0].Required = false }},
		{name: "duplicate parameter", mutate: func(tool *HTTPTool) { tool.Params = append(tool.Params, tool.Params[0]) }},
		{name: "forbidden public header", mutate: func(tool *HTTPTool) { tool.PublicHeaders = map[string]string{"Host": "elsewhere"} }},
		{name: "invalid header name", mutate: func(tool *HTTPTool) { tool.PublicHeaders = map[string]string{"Bad Header": "value"} }},
		{name: "secret collision", mutate: func(_ *HTTPTool) {}, secrets: map[string]string{"accept": "secret"}},
		{name: "header newline", mutate: func(_ *HTTPTool) {}, secrets: map[string]string{"X-Token": "bad\nvalue"}},
		{name: "header control character", mutate: func(_ *HTTPTool) {}, secrets: map[string]string{"X-Token": "bad\x00value"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tool := validHTTPTool()
			test.mutate(&tool)
			if err := ValidateHTTPTool(tool, test.secrets); !errors.Is(err, ErrValidation) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestValidateAndDecodeToolArguments(t *testing.T) {
	params := []ToolParam{{Name: "count", Type: ToolParamInteger, Required: true}, {Name: "ratio", Type: ToolParamNumber}, {Name: "active", Type: ToolParamBoolean}, {Name: "name", Type: ToolParamString}}
	args, err := ToolArguments(json.RawMessage(`{"count":2,"ratio":1.5,"active":true,"name":"Hue"}`))
	if err != nil || ValidateToolArguments(params, args) != nil {
		t.Fatalf("args=%#v err=%v", args, err)
	}
	for _, raw := range []string{`[]`, `null`, `{"count":1.5}`, `{"count":2,"extra":true}`, `{"name":"missing count"}`, `{} {}`, `{} invalid`} {
		args, decodeErr := ToolArguments(json.RawMessage(raw))
		if decodeErr == nil && ValidateToolArguments(params, args) == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestUniqueToolNameSanitizesBoundsAndCollisions(t *testing.T) {
	used := map[string]bool{}
	first := UniqueToolName("mcp_server_", strings.Repeat("tên dài ", 20), "one", used)
	second := UniqueToolName("mcp_server_", strings.Repeat("tên dài ", 20), "two", used)
	if first == second || len(first) > 64 || len(second) > 64 || !slugPattern.MatchString(first) || !slugPattern.MatchString(second) {
		t.Fatalf("first=%q second=%q", first, second)
	}
	if again := UniqueToolName("mcp_server_", strings.Repeat("tên dài ", 20), "two", map[string]bool{first: true}); again != second {
		t.Fatalf("non-deterministic name: %q != %q", again, second)
	}
}

func TestValidateMCPServerAllowedTools(t *testing.T) {
	allowed := []string{"one", "two"}
	server := MCPServer{Slug: "server", DisplayName: "Máy chủ", URL: "https://mcp.example.com", AllowedTools: &allowed}
	if err := ValidateMCPServer(server, map[string]string{"Authorization": "secret"}); err != nil {
		t.Fatal(err)
	}
	allowed = []string{"same", "same"}
	server.AllowedTools = &allowed
	if !errors.Is(ValidateMCPServer(server, nil), ErrValidation) {
		t.Fatal("duplicate allowed tool accepted")
	}
}

func TestStructuredToolParamsOnlyInNonGetBody(t *testing.T) {
	tool := validHTTPTool()
	tool.Params = append(tool.Params, ToolParam{Name: "session_flags", Type: ToolParamObject, Description: "Cờ phiên", Location: ToolParamBody})
	if err := ValidateHTTPTool(tool, nil); err != nil {
		t.Fatalf("object trong body của POST bị từ chối: %v", err)
	}
	tool.Params[len(tool.Params)-1].Type = ToolParamArray
	if err := ValidateHTTPTool(tool, nil); err != nil {
		t.Fatalf("array trong body của POST bị từ chối: %v", err)
	}
	for _, location := range []ToolParamLocation{ToolParamQuery, ToolParamPath} {
		rejected := validHTTPTool()
		rejected.Params = append(rejected.Params, ToolParam{Name: "session_flags", Type: ToolParamObject, Description: "Cờ phiên", Required: true, Location: location})
		if err := ValidateHTTPTool(rejected, nil); !errors.Is(err, ErrValidation) {
			t.Fatalf("chấp nhận object ở vị trí %q: %v", location, err)
		}
	}
	// A GET sends every argument as a query value, so nested JSON has nowhere to go.
	getTool := validHTTPTool()
	getTool.Method = HTTPToolGET
	getTool.Params = append(getTool.Params, ToolParam{Name: "session_flags", Type: ToolParamObject, Description: "Cờ phiên", Location: ToolParamBody})
	if err := ValidateHTTPTool(getTool, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("chấp nhận object trên GET: %v", err)
	}
}

func TestStructuredToolArgumentsAndSchema(t *testing.T) {
	params := []ToolParam{
		{Name: "session_flags", Type: ToolParamObject, Description: "screen_sharing, remote_app", Required: true, Location: ToolParamBody},
		{Name: "recent_events", Type: ToolParamArray, Description: "Danh sách mã sự kiện", Location: ToolParamBody},
	}
	args, err := ToolArguments(json.RawMessage(`{"session_flags":{"screen_sharing":true,"score":7},"recent_events":["NEW_DEVICE_LOGIN"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err = ValidateToolArguments(params, args); err != nil {
		t.Fatalf("đối số hợp lệ bị từ chối: %v", err)
	}
	for _, wrong := range []string{`{"session_flags":"{}"}`, `{"session_flags":["a"]}`, `{"session_flags":{},"recent_events":{"a":1}}`} {
		decoded, decodeErr := ToolArguments(json.RawMessage(wrong))
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if err = ValidateToolArguments(params, decoded); !errors.Is(err, ErrValidation) {
			t.Fatalf("chấp nhận đối số sai kiểu %s: %v", wrong, err)
		}
	}
	var schema map[string]any
	if err = json.Unmarshal(ToolJSONSchema(params), &schema); err != nil {
		t.Fatal(err)
	}
	properties, _ := schema["properties"].(map[string]any)
	flags, _ := properties["session_flags"].(map[string]any)
	if flags["type"] != "object" {
		t.Fatalf("schema object sai: %v", flags)
	}
	events, _ := properties["recent_events"].(map[string]any)
	if events["type"] != "array" {
		t.Fatalf("schema array sai: %v", events)
	}
	if _, ok := events["items"]; !ok {
		t.Fatal("schema array thiếu items, provider có thể từ chối")
	}
}

func TestObjectFieldsAndArrayItemTypeShapeSchema(t *testing.T) {
	params := []ToolParam{
		{Name: "session_flags", Type: ToolParamObject, Description: "Cờ phiên", Location: ToolParamBody, Fields: []ToolParamField{
			{Name: "screen_sharing", Type: ToolParamBoolean, Description: "Đang chia sẻ màn hình", Required: true},
			{Name: "remote_app", Type: ToolParamBoolean, Description: "Có ứng dụng điều khiển từ xa"},
		}},
		{Name: "recent_events", Type: ToolParamArray, Description: "Mã sự kiện", Location: ToolParamBody, ItemType: ToolParamString},
	}
	var schema map[string]any
	if err := json.Unmarshal(ToolJSONSchema(params), &schema); err != nil {
		t.Fatal(err)
	}
	properties, _ := schema["properties"].(map[string]any)
	flags, _ := properties["session_flags"].(map[string]any)
	inner, _ := flags["properties"].(map[string]any)
	sharing, _ := inner["screen_sharing"].(map[string]any)
	if sharing["type"] != "boolean" || sharing["description"] != "Đang chia sẻ màn hình" {
		t.Fatalf("trường lồng nhau sai: %v", inner)
	}
	required, _ := flags["required"].([]any)
	if len(required) != 1 || required[0] != "screen_sharing" {
		t.Fatalf("required của object sai: %v", flags["required"])
	}
	// The receiving system may accept keys this workspace has not declared.
	if _, closed := flags["additionalProperties"]; closed {
		t.Fatal("không nên khóa additionalProperties trên object lồng nhau")
	}
	events, _ := properties["recent_events"].(map[string]any)
	items, _ := events["items"].(map[string]any)
	if items["type"] != "string" {
		t.Fatalf("items của array sai: %v", events["items"])
	}
}

func TestStructuredArgumentsRespectDeclaredShape(t *testing.T) {
	params := []ToolParam{
		{Name: "session_flags", Type: ToolParamObject, Location: ToolParamBody, Fields: []ToolParamField{
			{Name: "screen_sharing", Type: ToolParamBoolean, Required: true},
			{Name: "device_age_days", Type: ToolParamInteger},
		}},
		{Name: "recent_events", Type: ToolParamArray, Location: ToolParamBody, ItemType: ToolParamString},
	}
	accepted := []string{
		`{"session_flags":{"screen_sharing":true},"recent_events":["A","B"]}`,
		`{"session_flags":{"screen_sharing":false,"device_age_days":3}}`,
		// Undeclared keys pass through, matching the generated schema.
		`{"session_flags":{"screen_sharing":true,"on_call":true}}`,
	}
	for _, raw := range accepted {
		args, err := ToolArguments(json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err = ValidateToolArguments(params, args); err != nil {
			t.Fatalf("từ chối đối số hợp lệ %s: %v", raw, err)
		}
	}
	rejected := []string{
		`{"session_flags":{}}`,
		`{"session_flags":{"screen_sharing":"yes"}}`,
		`{"session_flags":{"screen_sharing":true,"device_age_days":1.5}}`,
		`{"session_flags":{"screen_sharing":true},"recent_events":["A",7]}`,
	}
	for _, raw := range rejected {
		args, err := ToolArguments(json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err = ValidateToolArguments(params, args); !errors.Is(err, ErrValidation) {
			t.Fatalf("chấp nhận đối số sai %s: %v", raw, err)
		}
	}
}

func TestParamShapeDeclarationRules(t *testing.T) {
	base := validHTTPTool()
	cases := map[string]ToolParam{
		"fields trên kiểu string":    {Name: "x", Type: ToolParamString, Location: ToolParamBody, Fields: []ToolParamField{{Name: "a", Type: ToolParamString}}},
		"item_type trên kiểu object": {Name: "x", Type: ToolParamObject, Location: ToolParamBody, ItemType: ToolParamString},
		"kiểu phần tử không hợp lệ":  {Name: "x", Type: ToolParamArray, Location: ToolParamBody, ItemType: ToolParamArray},
		"trường lồng kiểu object":    {Name: "x", Type: ToolParamObject, Location: ToolParamBody, Fields: []ToolParamField{{Name: "a", Type: ToolParamObject}}},
		"tên trường không hợp lệ":    {Name: "x", Type: ToolParamObject, Location: ToolParamBody, Fields: []ToolParamField{{Name: "1bad", Type: ToolParamString}}},
		"tên trường trùng nhau":      {Name: "x", Type: ToolParamObject, Location: ToolParamBody, Fields: []ToolParamField{{Name: "a", Type: ToolParamString}, {Name: "a", Type: ToolParamString}}},
	}
	for name, param := range cases {
		tool := base
		tool.Params = append(append([]ToolParam{}, base.Params...), param)
		if err := ValidateHTTPTool(tool, nil); !errors.Is(err, ErrValidation) {
			t.Errorf("%s: được chấp nhận, lẽ ra phải từ chối (err=%v)", name, err)
		}
	}
}
