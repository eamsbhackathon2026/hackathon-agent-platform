package tooling

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

type resolvedEntry struct {
	httpTool    *domain.HTTPTool
	mcpServer   *domain.MCPServer
	mcpToolName string
	secrets     map[string]string
}

type resolvedToolSet struct {
	mu           sync.Mutex
	specs        []domain.ToolSpec
	entries      map[string]resolvedEntry
	sessions     map[uuid.UUID]outbound.MCPSession
	httpInvoker  outbound.HTTPToolInvoker
	mcpConnector outbound.MCPConnector
	closed       bool
}

func (s *resolvedToolSet) Specs() []domain.ToolSpec {
	result := make([]domain.ToolSpec, len(s.specs))
	copy(result, s.specs)
	return result
}

func (s *resolvedToolSet) Execute(ctx context.Context, call domain.ToolCall) domain.ToolResult {
	s.mu.Lock()
	entry, exists := s.entries[call.Name]
	closed := s.closed
	s.mu.Unlock()
	if !exists || closed {
		return domain.ToolResult{CallID: call.ID, Name: call.Name, Content: "Công cụ chưa được bật cho trợ lý này.", IsError: true}
	}
	if entry.httpTool != nil {
		invocation, err := s.httpInvoker.Invoke(ctx, *entry.httpTool, entry.secrets, call.Arguments)
		if err != nil {
			failure := toolFailure(err)
			return domain.ToolResult{CallID: call.ID, Name: call.Name, Content: failure.Message, IsError: true}
		}
		return domain.ToolResult{CallID: call.ID, Name: call.Name, Content: invocation.Body, IsError: invocation.IsError, Truncated: invocation.Truncated}
	}
	session, err := s.mcpSession(ctx, entry)
	if err != nil {
		failure := toolFailure(err)
		return domain.ToolResult{CallID: call.ID, Name: call.Name, Content: failure.Message, IsError: true}
	}
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	result, err := session.CallTool(callCtx, entry.mcpToolName, call.Arguments)
	if err != nil {
		failure := toolFailure(err)
		return domain.ToolResult{CallID: call.ID, Name: call.Name, Content: failure.Message, IsError: true}
	}
	result.CallID = call.ID
	result.Name = call.Name
	result.Content, result.Truncated = domain.TruncateUTF8(result.Content, 16*1024)
	return result
}

func (s *resolvedToolSet) mcpSession(ctx context.Context, entry resolvedEntry) (outbound.MCPSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("tool set is closed")
	}
	if session := s.sessions[entry.mcpServer.ID]; session != nil {
		return session, nil
	}
	session, err := s.mcpConnector.Connect(ctx, *entry.mcpServer, entry.secrets)
	if err != nil {
		return nil, err
	}
	s.sessions[entry.mcpServer.ID] = session
	return session, nil
}

func (s *resolvedToolSet) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	sessions := make([]outbound.MCPSession, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	s.sessions = map[uuid.UUID]outbound.MCPSession{}
	s.mu.Unlock()
	var result error
	for _, session := range sessions {
		result = errors.Join(result, session.Close())
	}
	return result
}

var _ outbound.ToolSet = (*resolvedToolSet)(nil)
