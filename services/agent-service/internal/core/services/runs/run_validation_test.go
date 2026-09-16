package runs

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

func TestValidateCommandRejectsInvalidCommandsAndAcceptsOwnedSession(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	base := fixture.command
	if _, _, err := fixture.service.validateCommand(t.Context(), fixture.principal, base); err != nil {
		t.Fatalf("valid command: %v", err)
	}
	sessionID := uuid.New()
	userID := fixture.principal.UserID
	if _, err := fixture.store.CreateSession(t.Context(), domain.Session{ID: sessionID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &userID, Title: "Chat", CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	owned := base
	owned.SessionID = &sessionID
	if _, _, err := fixture.service.validateCommand(t.Context(), fixture.principal, owned); err != nil {
		t.Fatalf("owned session: %v", err)
	}

	tests := []struct {
		name string
		edit func(*inbound.RunCommand)
		want error
	}{
		{name: "empty input", edit: func(c *inbound.RunCommand) { c.Input = "" }, want: domain.ErrValidation},
		{name: "both session selectors", edit: func(c *inbound.RunCommand) { c.SessionID = &sessionID; key := "key"; c.SessionKey = &key }, want: domain.ErrValidation},
		{name: "session key with user", edit: func(c *inbound.RunCommand) { key := "key"; c.SessionKey = &key }, want: domain.ErrValidation},
		{name: "async pending", edit: func(c *inbound.RunCommand) { c.Mode = domain.RunModeAsync }, want: domain.ErrNotImplemented},
		{name: "invalid mode", edit: func(c *inbound.RunCommand) { c.Mode = "other" }, want: domain.ErrValidation},
		{name: "metadata array", edit: func(c *inbound.RunCommand) { c.Metadata = json.RawMessage(`[]`) }, want: domain.ErrValidation},
		{name: "bad cursor-like metadata", edit: func(c *inbound.RunCommand) { c.Metadata = json.RawMessage(`{"x":`) }, want: domain.ErrValidation},
		{name: "missing agent", edit: func(c *inbound.RunCommand) { c.AgentID = uuid.New() }, want: domain.ErrNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := base
			test.edit(&command)
			if _, _, err := fixture.service.validateCommand(t.Context(), fixture.principal, command); !errors.Is(err, test.want) {
				t.Fatalf("err=%v want=%v", err, test.want)
			}
		})
	}

	otherID := uuid.New()
	otherSessionID := uuid.New()
	if _, err := fixture.store.CreateSession(t.Context(), domain.Session{ID: otherSessionID, AgentID: fixture.agent.ID, Source: domain.RunSourcePlayground, CreatedByUserID: &otherID, Title: "Other", CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	owned.SessionID = &otherSessionID
	if _, _, err := fixture.service.validateCommand(t.Context(), fixture.principal, owned); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("continued another user's session: %v", err)
	}
}

func TestValidateCommandSupportsAPIKeyExternalSessionAndPrivateClientFailures(t *testing.T) {
	fixture := newEngineFixture(t, nil, nil, nil)
	keyID := uuid.New()
	principal := domain.Principal{Kind: domain.PrincipalAPIKey, APIKeyID: keyID, Scopes: []string{"runs:write"}}
	key := "external"
	command := fixture.command
	command.SessionKey = &key
	if _, _, err := fixture.service.validateCommand(t.Context(), principal, command); err != nil {
		t.Fatalf("new external session: %v", err)
	}
	if _, err := fixture.service.client(t.Context(), domain.Provider{ID: uuid.New()}); !errors.Is(err, domain.ErrProviderNotConfigured) {
		t.Fatalf("missing provider key: %v", err)
	}
	if _, err := NewService(Dependencies{}); err == nil {
		t.Fatal("missing dependencies accepted")
	}
}

func TestSafeRunFailureMappings(t *testing.T) {
	tests := []struct {
		err  error
		code string
	}{
		{domain.ErrProviderNotConfigured, "provider_not_configured"},
		{domain.ErrProviderAuth, "provider_auth_failed"},
		{domain.ErrModelNotFound, "model_not_found"},
		{domain.ErrProviderRateLimited, "rate_limited"},
		{domain.ErrProviderBadRequest, "validation_failed"},
		{domain.ErrProviderUnreachable, "provider_unreachable"},
		{errors.New("unknown"), "internal"},
	}
	for _, test := range tests {
		if failure := safeRunFailure(test.err); failure == nil || failure.Code != test.code {
			t.Fatalf("err=%v failure=%+v", test.err, failure)
		}
	}
	if safeRunFailure(nil) != nil {
		t.Fatal("nil error produced a failure")
	}
}

func TestHistoryBuilderRepairsToolPairsAndDropsUnsafeResults(t *testing.T) {
	callID, secondID, name := "call", "second", "lookup"
	messages := []domain.Message{
		{Role: "tool", ToolCallID: &callID, ToolName: &name, Content: "dangling"},
		{Role: "user", Content: "question"},
		{Role: "assistant", Content: "", ToolCalls: []domain.ToolCall{{ID: callID, Name: name, Arguments: json.RawMessage(`{}`)}, {ID: secondID, Name: name, Arguments: json.RawMessage(`{}`)}}, ProviderMeta: []byte(`{"private":true}`)},
		{Role: "tool", ToolCallID: &callID, ToolName: &name, Content: "one"},
		{Role: "tool", ToolCallID: &callID, ToolName: &name, Content: "duplicate"},
		{Role: "user", Content: "continue"},
	}
	history := buildHistory(messages)
	if len(history) != 4 || history[0].Role != "user" || len(history[2].ToolResults) != 2 || string(history[1].ProviderMeta) != `{"private":true}` {
		t.Fatalf("history=%+v", history)
	}
	if history[2].ToolResults[0].Content != "one" || history[2].ToolResults[1].CallID != secondID || !history[2].ToolResults[1].IsError || history[2].ToolResults[1].Content != missingToolResultContent {
		t.Fatalf("tool results=%+v", history[2].ToolResults)
	}
}

func TestNormalizeToolCallIDsRejectsMalformedCalls(t *testing.T) {
	valid := domain.ToolCall{ID: "call-1", Name: "lookup", Arguments: json.RawMessage(`{"query":"weather"}`)}
	if _, ok := normalizeToolCallIDs([]domain.ToolCall{valid}, nil, nil, uuid.New(), 1); !ok {
		t.Fatal("valid tool call was rejected")
	}

	tests := []struct {
		name  string
		calls []domain.ToolCall
	}{
		{name: "missing id", calls: []domain.ToolCall{{Name: valid.Name, Arguments: valid.Arguments}}},
		{name: "missing name", calls: []domain.ToolCall{{ID: valid.ID, Arguments: valid.Arguments}}},
		{name: "invalid arguments", calls: []domain.ToolCall{{ID: valid.ID, Name: valid.Name, Arguments: json.RawMessage(`{"query":`)}}},
		{name: "array arguments", calls: []domain.ToolCall{{ID: valid.ID, Name: valid.Name, Arguments: json.RawMessage(`[]`)}}},
		{name: "null arguments", calls: []domain.ToolCall{{ID: valid.ID, Name: valid.Name, Arguments: json.RawMessage(`null`)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := normalizeToolCallIDs(test.calls, nil, nil, uuid.New(), 1); ok {
				t.Fatalf("accepted invalid calls: %+v", test.calls)
			}
		})
	}
}

func TestNormalizeToolCallIDsRepairsDuplicatesWithoutOpaqueMetadata(t *testing.T) {
	call := domain.ToolCall{ID: "duplicate", Name: "lookup", Arguments: json.RawMessage(`{}`)}
	history := []domain.ChatMessage{{Role: "assistant", ToolCalls: []domain.ToolCall{call}}}
	calls, ok := normalizeToolCallIDs([]domain.ToolCall{call, call}, nil, history, uuid.New(), 2)
	if !ok || calls[0].ID == call.ID || calls[1].ID == call.ID || calls[0].ID == calls[1].ID || len(calls[0].ID) > maxReplayToolCallIDLength || len(calls[1].ID) > maxReplayToolCallIDLength {
		t.Fatalf("calls=%+v ok=%v", calls, ok)
	}
	if _, ok = normalizeToolCallIDs([]domain.ToolCall{call}, []byte(`{"opaque":true}`), history, uuid.New(), 2); ok {
		t.Fatal("rewrote a duplicate ID tied to opaque provider metadata")
	}
	longNativeCall := domain.ToolCall{ID: "call_" + uuid.NewString(), Name: "lookup", Arguments: json.RawMessage(`{}`), ProviderMeta: []byte(`{"native":true}`)}
	nativeCalls, ok := normalizeToolCallIDs([]domain.ToolCall{longNativeCall}, []byte(`{"native":true}`), nil, uuid.New(), 1)
	if !ok || nativeCalls[0].ID != longNativeCall.ID {
		t.Fatalf("native provider ID was rejected or rewritten: calls=%+v ok=%v", nativeCalls, ok)
	}
}

func TestHistoryBuilderRepairsLegacyIDsAcrossTurns(t *testing.T) {
	callID, name := "reused", "lookup"
	toolMessage := func(content string) domain.Message {
		return domain.Message{Role: "tool", ToolCallID: &callID, ToolName: &name, Content: content}
	}
	messages := []domain.Message{
		{Role: "user", Content: "first"},
		{ID: uuid.New(), Role: "assistant", ToolCalls: []domain.ToolCall{{ID: callID, Name: name, Arguments: json.RawMessage(`{}`)}}, CreatedAt: time.Now()},
		toolMessage("one"),
		{Role: "user", Content: "second"},
		{ID: uuid.New(), Role: "assistant", ToolCalls: []domain.ToolCall{{ID: callID, Name: name, Arguments: json.RawMessage(`{}`)}}, CreatedAt: time.Now()},
		toolMessage("two"),
	}
	history := buildHistory(messages)
	firstID := history[1].ToolCalls[0].ID
	secondID := history[4].ToolCalls[0].ID
	if firstID != callID || secondID == callID || firstID == secondID || history[5].ToolResults[0].CallID != secondID {
		t.Fatalf("history=%+v", history)
	}
}

func TestHistoryBuilderDropsOpaqueDuplicateTurnWithoutDroppingText(t *testing.T) {
	callID, name := "native", "lookup"
	meta := json.RawMessage(`{"native":true}`)
	messages := []domain.Message{
		{Role: "user", Content: "first"},
		{ID: uuid.New(), Role: "assistant", ToolCalls: []domain.ToolCall{{ID: callID, Name: name, Arguments: json.RawMessage(`{}`), ProviderMeta: meta}}, ProviderMeta: meta},
		{Role: "tool", ToolCallID: &callID, ToolName: &name, Content: "one"},
		{Role: "user", Content: "second"},
		{ID: uuid.New(), Role: "assistant", Content: "partial answer", ToolCalls: []domain.ToolCall{{ID: callID, Name: name, Arguments: json.RawMessage(`{}`), ProviderMeta: meta}}, ProviderMeta: meta},
		{Role: "tool", ToolCallID: &callID, ToolName: &name, Content: "unsafe duplicate"},
		{Role: "user", Content: "continue"},
	}
	history := buildHistory(messages)
	if len(history) != 6 || len(history[1].ToolCalls) != 1 || history[4].Text != "partial answer" || len(history[4].ToolCalls) != 0 || len(history[4].ProviderMeta) != 0 || history[5].Text != "continue" {
		t.Fatalf("history=%+v", history)
	}
}

func TestAppendToolResultGroupsConsecutiveResults(t *testing.T) {
	history := appendToolResult(nil, domain.ToolResult{CallID: "one"})
	history = appendToolResult(history, domain.ToolResult{CallID: "two"})
	if len(history) != 1 || len(history[0].ToolResults) != 2 {
		t.Fatalf("history=%+v", history)
	}
}
