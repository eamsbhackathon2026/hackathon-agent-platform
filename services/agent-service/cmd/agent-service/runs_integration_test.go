//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestRunWiringSyncStreamHistoryAndAPIKeyIsolation(t *testing.T) {
	server, _ := identityTestServer(t)
	catalog := newCatalogHTTP(t, server)
	upstream := newCatalogUpstream(t, "openai_compatible")
	provider := catalog.provider("openai_compatible", upstream.server.URL, "Run connection")
	var agent gen.Agent
	catalog.request("POST", "/v1/agents", map[string]any{"name": "Run assistant", "provider_id": provider.Id, "model": "alpha", "max_output_tokens": 1}, 201, &agent)

	var run gen.Run
	catalog.request("POST", "/v1/agents/"+agent.Id.String()+"/runs", map[string]any{"input": map[string]string{"message": "Xin chào"}, "mode": "sync", "metadata": map[string]string{"case": "integration"}}, 200, &run)
	if run.Status != gen.RunStatusSucceeded || run.Output.MustGet() != "Hi" || run.Iterations != 1 || run.SessionId.String() == "" {
		t.Fatalf("sync run=%+v", run)
	}
	var spans gen.SpanPage
	catalog.request("GET", "/v1/runs/"+run.Id.String()+"/spans", nil, 200, &spans)
	if len(spans.Items) != 2 || spans.Items[0].Kind != gen.SpanKind("run") || spans.Items[1].Kind != gen.SpanKind("llm_call") {
		t.Fatalf("spans=%+v", spans.Items)
	}
	var messages gen.MessagePage
	catalog.request("GET", "/v1/sessions/"+run.SessionId.String()+"/messages?limit=1", nil, 200, &messages)
	if len(messages.Items) != 1 || messages.NextCursor.IsNull() || messages.Items[0].Role != gen.MessageRole("user") {
		t.Fatalf("first message page=%+v", messages)
	}
	catalog.request("GET", "/v1/sessions/"+run.SessionId.String()+"/messages?limit=1&cursor="+messages.NextCursor.MustGet(), nil, 200, &messages)
	if len(messages.Items) != 1 || !messages.NextCursor.IsNull() || messages.Items[0].Role != gen.MessageRole("assistant") {
		t.Fatalf("second message page=%+v", messages)
	}
	catalog.request("GET", "/v1/sessions/"+run.SessionId.String()+"/messages?run_id="+run.Id.String(), nil, 200, &messages)
	if len(messages.Items) != 2 {
		t.Fatalf("run message page=%+v", messages)
	}
	for _, message := range messages.Items {
		if message.RunId.IsNull() || message.RunId.MustGet() != run.Id {
			t.Fatalf("message is outside run %s: %+v", run.Id, message)
		}
	}

	response, err := runRequest(server.Client(), server.URL+"/v1/agents/"+agent.Id.String()+"/runs/stream", catalog.token, "", map[string]any{"input": map[string]string{"message": "Stream"}})
	if err != nil {
		t.Fatal(err)
	}
	stream, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || response.StatusCode != 200 || !bytes.Contains(stream, []byte("event: message.delta")) || !bytes.Contains(stream, []byte("event: run.completed")) {
		t.Fatalf("stream status=%d body=%s err=%v", response.StatusCode, stream, err)
	}

	member := catalog.member()
	var memberRun gen.Run
	member.request("POST", "/v1/agents/"+agent.Id.String()+"/runs", map[string]any{"input": map[string]string{"message": "Member"}, "mode": "sync"}, 200, &memberRun)
	member.request("GET", "/v1/runs/"+run.Id.String(), nil, 403, nil)
	var memberRuns gen.RunPage
	member.request("GET", "/v1/runs", nil, 200, &memberRuns)
	if len(memberRuns.Items) != 1 || memberRuns.Items[0].Id != memberRun.Id {
		t.Fatalf("member runs=%+v", memberRuns.Items)
	}
	var memberSessions gen.SessionPage
	member.request("GET", "/v1/sessions", nil, 200, &memberSessions)
	if len(memberSessions.Items) != 1 || memberSessions.Items[0].Id != memberRun.SessionId {
		t.Fatalf("member sessions=%+v", memberSessions.Items)
	}

	key, _ := identityRequest(t, server, "POST", "/v1/api-keys", map[string]any{"name": "Runner", "scopes": []string{"runs:write", "runs:read"}}, catalog.token, nil, "", 201)
	keyResponse, err := runRequest(server.Client(), server.URL+"/v1/agents/"+agent.Id.String()+"/runs", "", key.Key, map[string]any{"input": map[string]string{"message": "API"}, "mode": "sync", "session_key": "external-1"})
	if err != nil {
		t.Fatal(err)
	}
	var keyRun gen.Run
	if keyResponse.StatusCode != 200 || json.NewDecoder(keyResponse.Body).Decode(&keyRun) != nil {
		t.Fatalf("API key run status=%d", keyResponse.StatusCode)
	}
	_ = keyResponse.Body.Close()
	for _, path := range []string{"/v1/runs/" + keyRun.Id.String() + "/spans", "/v1/sessions"} {
		denied, requestErr := runRequest(server.Client(), server.URL+path, "", key.Key, nil)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		_ = denied.Body.Close()
		if denied.StatusCode != 403 {
			t.Fatalf("API key path %s status=%d", path, denied.StatusCode)
		}
	}
	allowed, err := runRequest(server.Client(), server.URL+"/v1/runs/"+keyRun.Id.String(), "", key.Key, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = allowed.Body.Close()
	if allowed.StatusCode != 200 {
		t.Fatalf("API key could not read own run: %d", allowed.StatusCode)
	}
	denied, err := runRequest(server.Client(), server.URL+"/v1/runs/"+run.Id.String(), "", key.Key, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = denied.Body.Close()
	if denied.StatusCode != 403 {
		t.Fatalf("API key read another principal's run: %d", denied.StatusCode)
	}
	keyPage, err := runRequest(server.Client(), server.URL+"/v1/runs", "", key.Key, nil)
	if err != nil {
		t.Fatal(err)
	}
	var keyRuns gen.RunPage
	if keyPage.StatusCode != 200 || json.NewDecoder(keyPage.Body).Decode(&keyRuns) != nil {
		t.Fatalf("API key run list status=%d", keyPage.StatusCode)
	}
	_ = keyPage.Body.Close()
	if len(keyRuns.Items) != 1 || keyRuns.Items[0].Id != keyRun.Id {
		t.Fatalf("API key runs=%+v", keyRuns.Items)
	}
	var ownerPage gen.RunPage
	catalog.request("GET", "/v1/runs?limit=1", nil, 200, &ownerPage)
	if len(ownerPage.Items) != 1 || ownerPage.NextCursor.IsNull() {
		t.Fatalf("owner first page=%+v", ownerPage)
	}
	firstRunID := ownerPage.Items[0].Id
	catalog.request("GET", "/v1/runs?limit=1&cursor="+ownerPage.NextCursor.MustGet(), nil, 200, &ownerPage)
	if len(ownerPage.Items) != 1 || ownerPage.Items[0].Id == firstRunID {
		t.Fatalf("owner second page=%+v", ownerPage)
	}
	catalog.request("GET", "/v1/runs?source=api", nil, 200, &ownerPage)
	if len(ownerPage.Items) != 1 || ownerPage.Items[0].Id != keyRun.Id {
		t.Fatalf("filtered API runs=%+v", ownerPage.Items)
	}
	catalog.request("DELETE", "/v1/sessions/"+keyRun.SessionId.String(), nil, 204, nil)
	reusedResponse, err := runRequest(server.Client(), server.URL+"/v1/agents/"+agent.Id.String()+"/runs", "", key.Key, map[string]any{"input": map[string]string{"message": "API again"}, "mode": "sync", "session_key": "external-1"})
	if err != nil {
		t.Fatal(err)
	}
	var reusedRun gen.Run
	if reusedResponse.StatusCode != 200 || json.NewDecoder(reusedResponse.Body).Decode(&reusedRun) != nil {
		t.Fatalf("reused API session status=%d", reusedResponse.StatusCode)
	}
	_ = reusedResponse.Body.Close()
	if reusedRun.SessionId == keyRun.SessionId {
		t.Fatal("soft-deleted API session was reused")
	}
}

func runRequest(client *http.Client, url, token, apiKey string, body any) (*http.Response, error) {
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(raw)
	}
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	request, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Origin", identityOrigin)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if apiKey != "" {
		request.Header.Set("X-API-Key", apiKey)
	}
	return client.Do(request)
}
