package domain

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/http/httpguts"
)

var (
	slugPattern      = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	paramNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	placeholder      = regexp.MustCompile(`\{([^{}]+)\}`)
)

// runeOutsideOneLine reports whether value carries a rune that cannot sit in a
// single line of running text: a control rune (Cc, which includes \n, \r, \t and
// U+0085), a line or paragraph separator (Zl, Zp — U+2028, U+2029), or an
// invisible formatting rune (Cf, which includes the bidi overrides that can make
// a label render as something other than what it stores). Joiners used by emoji
// sequences fall under Cf too, so a label cannot carry a composed emoji; a step
// label is a sentence, and that trade is worth closing the spoofing hole.
func runeOutsideOneLine(value string) bool {
	for _, r := range value {
		if unicode.In(r, unicode.Cc, unicode.Cf, unicode.Zl, unicode.Zp) {
			return true
		}
	}
	return false
}

// ValidateHTTPTool validates a complete configuration without decrypted secrets.
func ValidateHTTPTool(tool HTTPTool, secretHeaders map[string]string) error {
	if !slugPattern.MatchString(tool.Slug) {
		return Invalid("slug", "Mã công cụ chỉ gồm chữ, số, gạch ngang hoặc gạch dưới và dài tối đa 64 ký tự.")
	}
	if err := catalogText("display_name", tool.DisplayName, 1, 200); err != nil {
		return err
	}
	// Giới hạn đếm trên phần đã cắt khoảng trắng hai đầu, vì người đọc chỉ thấy
	// phần đó — bên gọi cũng cắt như vậy trước khi hiện.
	if err := catalogText("step_label", strings.TrimSpace(tool.StepLabel), 0, 80); err != nil {
		return err
	}
	// Kiểm tra trên chuỗi GỐC: TrimSpace đã nuốt mất ký tự xuống dòng ở hai đầu.
	// Nhãn bước hiện trên một dòng của danh sách đang chạy, nên ký tự điều khiển,
	// ký tự tách dòng và ký tự định dạng vô hình chỉ làm vỡ bố cục hoặc đảo chiều
	// chữ. Chuỗi ở đây không phải lúc nào cũng do người vận hành gõ: bundle công
	// cụ nhập từ môi trường khác cũng đi qua đúng cửa này.
	if runeOutsideOneLine(tool.StepLabel) {
		return Invalid("step_label", "Nhãn bước phải nằm gọn trên một dòng và không chứa ký tự vô hình.")
	}
	if err := catalogText("description", tool.Description, 0, 4000); err != nil {
		return err
	}
	switch tool.Method {
	case HTTPToolGET, HTTPToolPOST, HTTPToolPUT, HTTPToolPATCH, HTTPToolDELETE:
	default:
		return Invalid("method", "Phương thức HTTP không được hỗ trợ.")
	}
	if err := catalogText("url_template", tool.URLTemplate, 1, 2048); err != nil {
		return err
	}
	if tool.ConnectionID != nil {
		if err := validateConnectionPath(tool.URLTemplate); err != nil {
			return err
		}
	}
	if tool.TimeoutSeconds < 1 || tool.TimeoutSeconds > 60 {
		return Invalid("timeout_seconds", "Thời gian chờ phải từ 1 đến 60 giây.")
	}
	if len(tool.Params) > 100 {
		return Invalid("params", "Một công cụ có tối đa 100 tham số.")
	}
	params := make(map[string]ToolParam, len(tool.Params))
	for _, param := range tool.Params {
		if !paramNamePattern.MatchString(param.Name) || len(param.Name) > 64 {
			return Invalid("params", "Tên tham số không hợp lệ hoặc dài quá 64 ký tự.")
		}
		if _, exists := params[param.Name]; exists {
			return Invalid("params", "Tên tham số không được trùng nhau.")
		}
		if err := catalogText("params", param.Description, 0, 4000); err != nil {
			return err
		}
		switch param.Type {
		case ToolParamString, ToolParamNumber, ToolParamInteger, ToolParamBoolean, ToolParamObject, ToolParamArray:
		default:
			return Invalid("params", "Kiểu tham số không hợp lệ.")
		}
		switch param.Location {
		case ToolParamPath:
			if !param.Required {
				return Invalid("params", "Tham số trong đường dẫn phải là bắt buộc.")
			}
		case ToolParamQuery, ToolParamBody:
		default:
			return Invalid("params", "Vị trí tham số không hợp lệ.")
		}
		// Path and query values are serialized as a single string, and a GET request sends
		// every argument as a query value, so nested JSON would have nowhere to go.
		if param.Type.Structured() && (param.Location != ToolParamBody || tool.Method == HTTPToolGET) {
			return Invalid("params", "Tham số kiểu object hoặc array chỉ dùng được ở phần thân của yêu cầu khác GET.")
		}
		if err := validateParamShape(param); err != nil {
			return err
		}
		params[param.Name] = param
	}
	found := map[string]bool{}
	for _, match := range placeholder.FindAllStringSubmatch(tool.URLTemplate, -1) {
		name := match[1]
		param, ok := params[name]
		if !ok || param.Location != ToolParamPath || !param.Required {
			return Invalid("url_template", "Mỗi biến trong địa chỉ phải có tham số đường dẫn bắt buộc tương ứng.")
		}
		found[name] = true
	}
	for name, param := range params {
		if param.Location == ToolParamPath && !found[name] {
			return Invalid("params", "Mỗi tham số đường dẫn phải xuất hiện trong địa chỉ công cụ.")
		}
	}
	return validateToolHeaders(tool.PublicHeaders, secretHeaders)
}

// ValidateMCPServer validates stored configuration and replacement secret headers.
func ValidateMCPServer(server MCPServer, secretHeaders map[string]string) error {
	if !slugPattern.MatchString(server.Slug) {
		return Invalid("slug", "Mã máy chủ chỉ gồm chữ, số, gạch ngang hoặc gạch dưới và dài tối đa 64 ký tự.")
	}
	if err := catalogText("display_name", server.DisplayName, 1, 200); err != nil {
		return err
	}
	if err := catalogText("url", server.URL, 1, 2048); err != nil {
		return err
	}
	if server.AllowedTools != nil {
		seen := map[string]bool{}
		for _, name := range *server.AllowedTools {
			if strings.TrimSpace(name) == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 200 || seen[name] {
				return Invalid("allowed_tools", "Danh sách công cụ được phép không hợp lệ hoặc bị trùng.")
			}
			seen[name] = true
		}
	}
	return validateToolHeaders(nil, secretHeaders)
}

func validateToolHeaders(public, secret map[string]string) error {
	if len(public) > 50 || len(secret) > 50 {
		return Invalid("headers", "Mỗi nhóm có tối đa 50 header.")
	}
	seen := map[string]bool{}
	for _, headers := range []map[string]string{public, secret} {
		for name, value := range headers {
			canonical := strings.ToLower(name)
			if !httpguts.ValidHeaderFieldName(name) || !httpguts.ValidHeaderFieldValue(value) || forbiddenToolHeader(canonical) || !utf8.ValidString(value) {
				return Invalid("headers", "Tên hoặc giá trị header không hợp lệ.")
			}
			if seen[canonical] {
				return Invalid("headers", "Header công khai và bí mật không được trùng tên.")
			}
			seen[canonical] = true
		}
	}
	return nil
}

func forbiddenToolHeader(name string) bool {
	switch name {
	case "host", "content-length", "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

// ToolJSONSchema generates a closed object schema from declared parameters.
func ToolJSONSchema(params []ToolParam) json.RawMessage {
	properties := make(map[string]any, len(params))
	required := []string{}
	for _, param := range params {
		property := map[string]any{"type": string(param.Type), "description": param.Description}
		if param.Type == ToolParamArray {
			// Providers reject an array schema that does not say what it contains, so an
			// undeclared element type still produces a schema that accepts anything.
			items := map[string]any{}
			if param.ItemType != "" {
				items["type"] = string(param.ItemType)
			}
			property["items"] = items
		}
		if param.Type == ToolParamObject && len(param.Fields) > 0 {
			fields := make(map[string]any, len(param.Fields))
			fieldRequired := []string{}
			for _, field := range param.Fields {
				fields[field.Name] = map[string]any{"type": string(field.Type), "description": field.Description}
				if field.Required {
					fieldRequired = append(fieldRequired, field.Name)
				}
			}
			sort.Strings(fieldRequired)
			property["properties"] = fields
			if len(fieldRequired) > 0 {
				property["required"] = fieldRequired
			}
			// No additionalProperties:false here. The declaration describes the keys this
			// workspace knows about, while the receiving system may accept more.
		}
		properties[param.Name] = property
		if param.Required {
			required = append(required, param.Name)
		}
	}
	sort.Strings(required)
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	value, _ := json.Marshal(schema)
	return value
}

// validateParamShape checks the optional inner declaration of a structured parameter.
func validateParamShape(param ToolParam) error {
	if len(param.Fields) > 0 && param.Type != ToolParamObject {
		return Invalid("params", "Chỉ tham số kiểu object mới khai được danh sách trường.")
	}
	if param.ItemType != "" && param.Type != ToolParamArray {
		return Invalid("params", "Chỉ tham số kiểu array mới khai được kiểu phần tử.")
	}
	switch param.ItemType {
	case "", ToolParamString, ToolParamNumber, ToolParamInteger, ToolParamBoolean, ToolParamObject:
	default:
		return Invalid("params", "Kiểu phần tử không hợp lệ.")
	}
	if len(param.Fields) > 50 {
		return Invalid("params", "Một tham số có tối đa 50 trường.")
	}
	seen := make(map[string]bool, len(param.Fields))
	for _, field := range param.Fields {
		if !paramNamePattern.MatchString(field.Name) || len(field.Name) > 64 {
			return Invalid("params", "Tên trường không hợp lệ hoặc dài quá 64 ký tự.")
		}
		if seen[field.Name] {
			return Invalid("params", "Tên trường không được trùng nhau.")
		}
		seen[field.Name] = true
		if err := catalogText("params", field.Description, 0, 4000); err != nil {
			return err
		}
		switch field.Type {
		case ToolParamString, ToolParamNumber, ToolParamInteger, ToolParamBoolean:
		default:
			return Invalid("params", "Trường bên trong chỉ nhận kiểu văn bản, số hoặc đúng/sai.")
		}
	}
	return nil
}

func decodeToolArguments(raw json.RawMessage) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var args map[string]any
	if err := decoder.Decode(&args); err != nil || args == nil {
		return nil, Invalid("args", "Đối số công cụ phải là một object JSON hợp lệ.")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, Invalid("args", "Đối số công cụ phải là một object JSON duy nhất.")
	}
	return args, nil
}

// ValidateToolArguments enforces declared names, primitive types and required fields.
func ValidateToolArguments(params []ToolParam, args map[string]any) error {
	declared := make(map[string]ToolParam, len(params))
	for _, param := range params {
		declared[param.Name] = param
		value, ok := args[param.Name]
		if !ok {
			if param.Required {
				return Invalid("args."+param.Name, "Thiếu tham số bắt buộc.")
			}
			continue
		}
		if !validToolArgumentType(param.Type, value) {
			return Invalid("args."+param.Name, "Giá trị không đúng kiểu đã khai báo.")
		}
		if err := validateArgumentShape(param, value); err != nil {
			return err
		}
	}
	for name := range args {
		if _, ok := declared[name]; !ok {
			return Invalid("args."+name, "Tham số không được khai báo.")
		}
	}
	return nil
}

// validateArgumentShape checks a structured value against the parameter's inner
// declaration. Undeclared object keys pass through, matching the generated schema.
func validateArgumentShape(param ToolParam, value any) error {
	switch param.Type {
	case ToolParamObject:
		members, _ := value.(map[string]any)
		for _, field := range param.Fields {
			member, ok := members[field.Name]
			if !ok {
				if field.Required {
					return Invalid("args."+param.Name+"."+field.Name, "Thiếu trường bắt buộc.")
				}
				continue
			}
			if !validToolArgumentType(field.Type, member) {
				return Invalid("args."+param.Name+"."+field.Name, "Giá trị không đúng kiểu đã khai báo.")
			}
		}
	case ToolParamArray:
		if param.ItemType == "" {
			return nil
		}
		items, _ := value.([]any)
		for index, item := range items {
			if !validToolArgumentType(param.ItemType, item) {
				return Invalid("args."+param.Name+"."+strconv.Itoa(index), "Phần tử không đúng kiểu đã khai báo.")
			}
		}
	}
	return nil
}

func validToolArgumentType(kind ToolParamType, value any) bool {
	switch kind {
	case ToolParamString:
		_, ok := value.(string)
		return ok
	case ToolParamBoolean:
		_, ok := value.(bool)
		return ok
	case ToolParamNumber:
		_, ok := value.(json.Number)
		return ok
	case ToolParamInteger:
		n, ok := value.(json.Number)
		if !ok {
			return false
		}
		_, err := n.Int64()
		return err == nil
	case ToolParamObject:
		_, ok := value.(map[string]any)
		return ok
	case ToolParamArray:
		_, ok := value.([]any)
		return ok
	default:
		return false
	}
}
