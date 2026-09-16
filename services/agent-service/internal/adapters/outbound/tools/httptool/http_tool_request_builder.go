package httptool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"agent-platform/services/agent-service/internal/core/domain"
)

func buildRequest(ctxURL string, tool domain.HTTPTool, secrets map[string]string, args map[string]any) (*http.Request, error) {
	rawURL := ctxURL
	query := url.Values{}
	body := map[string]any{}
	for _, param := range tool.Params {
		value, ok := args[param.Name]
		if !ok {
			continue
		}
		switch {
		case param.Location == domain.ToolParamPath:
			rawURL = strings.ReplaceAll(rawURL, "{"+param.Name+"}", url.PathEscape(argumentString(value)))
		case param.Location == domain.ToolParamQuery || tool.Method == domain.HTTPToolGET:
			query.Set(param.Name, argumentString(value))
		default:
			body[param.Name] = value
		}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, domain.ErrProviderBadRequest
	}
	parsed.RawQuery = query.Encode()
	var payload *bytes.Reader
	if len(body) > 0 {
		encoded, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, domain.ErrProviderBadRequest
		}
		payload = bytes.NewReader(encoded)
	} else {
		payload = bytes.NewReader(nil)
	}
	request, err := http.NewRequest(string(tool.Method), parsed.String(), payload)
	if err != nil {
		return nil, domain.ErrProviderBadRequest
	}
	for name, value := range tool.PublicHeaders {
		request.Header.Set(name, value)
	}
	for name, value := range secrets {
		request.Header.Set(name, value)
	}
	if len(body) > 0 && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func argumentString(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	case bool:
		if value {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(value)
	}
}
