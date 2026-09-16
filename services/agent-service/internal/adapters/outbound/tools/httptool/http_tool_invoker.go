// Package httptool executes saved HTTP tools through an egress-guarded client.
package httptool

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

const maxResponseBytes = 16 * 1024

// Invoker owns no credentials and delegates destination validation to its client transport.
type Invoker struct{ client *http.Client }

// New creates an HTTP tool invoker backed by an egress-guarded client.
func New(client *http.Client) (*Invoker, error) {
	if client == nil || client.Transport == nil {
		return nil, errors.New("guarded HTTP client is required")
	}
	return &Invoker{client: client}, nil
}

// Invoke validates arguments, applies a per-tool timeout and returns safe text only.
func (i *Invoker) Invoke(ctx context.Context, tool domain.HTTPTool, secrets map[string]string, raw json.RawMessage) (domain.HTTPToolInvocation, error) {
	args, err := domain.ToolArguments(raw)
	if err != nil {
		return domain.HTTPToolInvocation{}, err
	}
	if err = domain.ValidateToolArguments(tool.Params, args); err != nil {
		return domain.HTTPToolInvocation{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(tool.TimeoutSeconds)*time.Second)
	defer cancel()
	request, err := buildRequest(tool.URLTemplate, tool, secrets, args)
	if err != nil {
		return domain.HTTPToolInvocation{}, err
	}
	request = request.WithContext(ctx)
	response, err := i.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return domain.HTTPToolInvocation{}, ctx.Err()
		}
		return domain.HTTPToolInvocation{}, err
	}
	defer func() { _ = response.Body.Close() }()
	status := response.StatusCode
	result := domain.HTTPToolInvocation{StatusCode: &status, IsError: status < 200 || status >= 300}
	if !textualContent(response.Header.Get("Content-Type")) {
		result.Body = "[nội dung nhị phân bị bỏ qua]"
		return result, nil
	}
	value, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return domain.HTTPToolInvocation{}, err
	}
	result.Truncated = len(value) > maxResponseBytes
	if result.Truncated {
		value = value[:maxResponseBytes]
	}
	body, truncatedUTF8 := domain.TruncateUTF8(strings.ToValidUTF8(string(value), "�"), maxResponseBytes)
	result.Body = body
	result.Truncated = result.Truncated || truncatedUTF8
	return result, nil
}

func textualContent(value string) bool {
	if value == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" || mediaType == "application/xml" || strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml")
}

var _ outbound.HTTPToolInvoker = (*Invoker)(nil)
