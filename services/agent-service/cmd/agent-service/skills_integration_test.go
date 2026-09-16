//go:build integration

package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

func TestSkillHubHTTPAPIAndRunContext(t *testing.T) {
	server, _ := identityTestServer(t)
	catalog := newCatalogHTTP(t, server)
	var llmMu sync.Mutex
	var llmRequests [][]byte
	llmServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Errorf("LLM path=%s", request.URL.Path)
		}
		llmRequest, _ := io.ReadAll(request.Body)
		llmMu.Lock()
		llmRequests = append(llmRequests, append([]byte(nil), llmRequest...))
		llmMu.Unlock()
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(writer, "data: {\"id\":\"skill\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Done\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer llmServer.Close()

	provider := catalog.provider("openai_compatible", llmServer.URL+"/v1", "Skill connection")
	var agent gen.Agent
	catalog.request("POST", "/v1/agents", map[string]any{"name": "Writing assistant", "provider_id": provider.Id, "model": "test", "system_prompt": "Base guidance", "max_output_tokens": 32}, 201, &agent)

	markdown := []byte("---\nname: Clear writing\ndescription: Improve readability.\n---\nUse active voice.")
	created := uploadSkill(t, server, catalog.token, "clear-writing.md", markdown, 201)
	if created.Name != "Clear writing" || created.Content != string(markdown) || created.SourceType != gen.SkillSourceTypeMarkdown {
		t.Fatalf("created=%+v", created)
	}
	zipCreated := uploadSkill(t, server, catalog.token, "summaries.zip", zipSkill(t, "summaries/SKILL.md", "# Summaries\n\nLead with the outcome."), 201)
	if zipCreated.Name != "Summaries" || zipCreated.SourceType != gen.SkillSourceTypeZip {
		t.Fatalf("zip created=%+v", zipCreated)
	}

	var bindings gen.AgentSkillBindings
	catalog.request("PUT", "/v1/agents/"+agent.Id.String()+"/skills", map[string]any{"skill_ids": []string{created.Id.String()}}, 200, &bindings)
	if len(bindings.SkillIds) != 1 || bindings.SkillIds[0] != created.Id {
		t.Fatalf("bindings=%+v", bindings)
	}
	var run gen.Run
	catalog.request("POST", "/v1/agents/"+agent.Id.String()+"/runs", map[string]any{"input": map[string]string{"message": "Rewrite this"}, "mode": "sync"}, 200, &run)
	if run.Status != gen.RunStatusSucceeded {
		t.Fatalf("run=%+v", run)
	}
	assertSkillRequests(t, &llmMu, &llmRequests, 1)

	streamBody, _ := json.Marshal(map[string]any{"input": map[string]string{"message": "Stream this"}})
	streamRequest, err := http.NewRequest(http.MethodPost, server.URL+"/v1/agents/"+agent.Id.String()+"/runs/stream", bytes.NewReader(streamBody))
	if err != nil {
		t.Fatal(err)
	}
	streamRequest.Header.Set("Authorization", "Bearer "+catalog.token)
	streamRequest.Header.Set("Origin", identityOrigin)
	streamRequest.Header.Set("Content-Type", "application/json")
	streamResponse, err := server.Client().Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	streamEvents, readErr := io.ReadAll(streamResponse.Body)
	_ = streamResponse.Body.Close()
	if readErr != nil || streamResponse.StatusCode != http.StatusOK || !bytes.Contains(streamEvents, []byte("event: run.completed")) {
		t.Fatalf("stream status=%d events=%s err=%v", streamResponse.StatusCode, streamEvents, readErr)
	}
	assertSkillRequests(t, &llmMu, &llmRequests, 2)

	var accepted gen.RunAccepted
	catalog.request("POST", "/v1/agents/"+agent.Id.String()+"/runs", map[string]any{"input": map[string]string{"message": "Process later"}, "mode": "async"}, 202, &accepted)
	var asyncRun gen.Run
	for range 100 {
		catalog.request("GET", "/v1/runs/"+accepted.RunId.String(), nil, 200, &asyncRun)
		if asyncRun.Status == gen.RunStatusSucceeded || asyncRun.Status == gen.RunStatusFailed || asyncRun.Status == gen.RunStatusCancelled {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if asyncRun.Status != gen.RunStatusSucceeded {
		t.Fatalf("async run=%+v", asyncRun)
	}
	assertSkillRequests(t, &llmMu, &llmRequests, 3)

	member := catalog.member()
	uploadSkill(t, server, member.token, "forbidden.md", []byte("# Forbidden"), 403)
	member.request("DELETE", "/v1/skills/"+zipCreated.Id.String(), nil, 403, nil)
	member.request("PUT", "/v1/agents/"+agent.Id.String()+"/skills", map[string]any{"skill_ids": []string{zipCreated.Id.String()}}, 403, nil)
	member.request("GET", "/v1/skills/"+created.Id.String(), nil, 200, nil)

	catalog.request("DELETE", "/v1/skills/"+created.Id.String(), nil, 204, nil)
	catalog.request("GET", "/v1/agents/"+agent.Id.String()+"/skills", nil, 200, &bindings)
	if len(bindings.SkillIds) != 0 {
		t.Fatalf("binding did not cascade: %+v", bindings)
	}
}

func assertSkillRequests(t *testing.T, mu *sync.Mutex, requests *[][]byte, want int) {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	if len(*requests) != want {
		t.Fatalf("LLM requests=%d want=%d", len(*requests), want)
	}
	for _, request := range *requests {
		requestText := string(request)
		if !strings.Contains(requestText, "Base guidance") || !strings.Contains(requestText, "<skill_instructions name=\\\"Clear writing\\\">") || !strings.Contains(requestText, "Use active voice.") || strings.Contains(requestText, "Improve readability.") {
			t.Fatalf("skill context mismatch: %s", requestText)
		}
	}
}

func uploadSkill(t *testing.T, server *httptest.Server, token, filename string, content []byte, want int) gen.Skill {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/skills", &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", identityOrigin)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != want {
		t.Fatalf("upload status=%d want=%d body=%s", response.StatusCode, want, data)
	}
	var skill gen.Skill
	if err = json.Unmarshal(data, &skill); err != nil {
		t.Fatal(err)
	}
	return skill
}

func zipSkill(t *testing.T, name, content string) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	part, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}
