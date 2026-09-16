// Package mcpclient adapts the MCP Go SDK behind core ports.
package mcpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

const (
	maxCachedTools       = 500
	maxCachedCatalogSize = 256 * 1024
)

// Connector opens Streamable HTTP sessions using one guarded base transport.
type Connector struct{ client *http.Client }

// New creates an MCP connector backed by an egress-guarded HTTP client.
func New(client *http.Client) (*Connector, error) {
	if client == nil || client.Transport == nil {
		return nil, errors.New("guarded MCP HTTP client is required")
	}
	return &Connector{client: client}, nil
}

// Connect opens one Streamable HTTP session with decrypted request headers.
func (c *Connector) Connect(ctx context.Context, server domain.MCPServer, secrets map[string]string) (outbound.MCPSession, error) {
	transportClient := *c.client
	headers := make(map[string]string, len(secrets))
	for name, value := range secrets {
		headers[name] = value
	}
	transportClient.Transport = headerRoundTripper{next: c.client.Transport, headers: headers}
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "agent-platform", Version: "0.1.0"}, nil)
	session, err := client.Connect(connectCtx, &mcp.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &transportClient, MaxRetries: 1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		return nil, err
	}
	return &sessionAdapter{session: session}, nil
}

type sessionAdapter struct{ session *mcp.ClientSession }

func (s *sessionAdapter) ListTools(ctx context.Context) ([]domain.MCPTool, error) {
	result := []domain.MCPTool{}
	cursor := ""
	seen := map[string]bool{}
	totalCatalogSize := 0
	for {
		page, err := s.session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		for _, tool := range page.Tools {
			if tool == nil || strings.TrimSpace(tool.Name) == "" || !utf8.ValidString(tool.Name) || utf8.RuneCountInString(tool.Name) > 200 || !utf8.ValidString(tool.Description) || utf8.RuneCountInString(tool.Description) > 4000 || seen[tool.Name] || len(result) >= maxCachedTools {
				return nil, domain.ErrProviderBadRequest
			}
			schema, err := normalizeSchema(tool.InputSchema)
			if err != nil {
				return nil, err
			}
			totalCatalogSize += len(tool.Name) + len(tool.Description) + len(schema)
			if totalCatalogSize > maxCachedCatalogSize {
				return nil, domain.ErrProviderBadRequest
			}
			seen[tool.Name] = true
			result = append(result, domain.MCPTool{Name: tool.Name, Description: tool.Description, InputSchema: schema})
		}
		if page.NextCursor == "" {
			return result, nil
		}
		if page.NextCursor == cursor {
			return nil, domain.ErrProviderBadRequest
		}
		cursor = page.NextCursor
	}
}

func normalizeSchema(value any) (json.RawMessage, error) {
	if value == nil {
		return json.RawMessage(`{"type":"object","properties":{}}`), nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, domain.ErrProviderBadRequest
	}
	var object map[string]any
	if json.Unmarshal(encoded, &object) != nil || object == nil {
		return nil, domain.ErrProviderBadRequest
	}
	return encoded, nil
}

func (s *sessionAdapter) CallTool(ctx context.Context, name string, raw json.RawMessage) (domain.ToolResult, error) {
	args, err := domain.ToolArguments(raw)
	if err != nil {
		return domain.ToolResult{}, err
	}
	result, err := s.session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return domain.ToolResult{}, err
	}
	parts := []string{}
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	if len(parts) == 0 && result.StructuredContent != nil {
		if value, marshalErr := json.Marshal(result.StructuredContent); marshalErr == nil {
			parts = append(parts, string(value))
		}
	}
	if len(parts) == 0 && len(result.Content) > 0 {
		parts = append(parts, "[nội dung không phải văn bản bị bỏ qua]")
	}
	return domain.ToolResult{Name: name, Content: strings.Join(parts, "\n"), IsError: result.IsError}, nil
}

func (s *sessionAdapter) Close() error {
	if s.session == nil {
		return nil
	}
	if err := s.session.Close(); err != nil {
		return fmt.Errorf("close MCP session: %w", err)
	}
	return nil
}

var _ outbound.MCPConnector = (*Connector)(nil)
var _ outbound.MCPSession = (*sessionAdapter)(nil)
