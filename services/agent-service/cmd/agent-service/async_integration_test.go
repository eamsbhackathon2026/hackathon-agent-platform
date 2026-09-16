//go:build integration

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

type receivedWebhook struct {
	headers http.Header
	body    []byte
}

func TestAsyncRunDeliversSignedWebhookAndReplaysIdempotency(t *testing.T) {
	server, _ := identityTestServer(t)
	catalog := newCatalogHTTP(t, server)
	upstream := newCatalogUpstream(t, "openai_compatible")
	provider := catalog.provider("openai_compatible", upstream.server.URL, "Async connection")
	var agent gen.Agent
	catalog.request("POST", "/v1/agents", map[string]any{"name": "Async assistant", "provider_id": provider.Id, "model": "alpha", "max_output_tokens": 1}, 201, &agent)
	key, _ := identityRequest(t, server, "POST", "/v1/api-keys", map[string]any{"name": "Async caller", "scopes": []string{"runs:write", "runs:read"}}, catalog.token, nil, "", 201)

	received := make(chan receivedWebhook, 4)
	var receiverStatus atomic.Int32
	receiverStatus.Store(http.StatusNoContent)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- receivedWebhook{headers: r.Header.Clone(), body: body}
		w.WriteHeader(int(receiverStatus.Load()))
	}))
	defer receiver.Close()
	body := map[string]any{"input": map[string]string{"message": "Async"}, "mode": "async", "webhook_url": receiver.URL}
	first := postAsyncRun(t, server, agent.Id.String(), key.Key, "request-1", body, http.StatusAccepted)
	if first.Location == "" || first.Accepted.Status != gen.RunStatusQueued {
		t.Fatalf("accepted=%+v", first)
	}

	var callback receivedWebhook
	select {
	case callback = <-received:
	case <-time.After(10 * time.Second):
		t.Fatal("không nhận được webhook")
	}
	verifyWebhook(t, callback, key.WebhookSecret)
	var payload gen.RunWebhookPayload
	if json.Unmarshal(callback.body, &payload) != nil || payload.Type != gen.RunWebhookPayloadTypeRunCompleted || payload.Data.Id != first.Accepted.RunId {
		t.Fatalf("payload=%s", callback.body)
	}

	response, err := runRequest(server.Client(), server.URL+first.Location, "", key.Key, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	var completed gen.Run
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&completed) != nil || completed.Status != gen.RunStatusSucceeded {
		t.Fatalf("run status=%d run=%+v", response.StatusCode, completed)
	}

	replayed := postAsyncRun(t, server, agent.Id.String(), key.Key, "request-1", body, http.StatusAccepted)
	if replayed.Accepted.RunId != first.Accepted.RunId || replayed.Accepted.Status != gen.RunStatusSucceeded {
		t.Fatalf("replay=%+v first=%+v", replayed, first)
	}
	changed := map[string]any{"input": map[string]string{"message": "Different"}, "mode": "async", "webhook_url": receiver.URL}
	postAsyncRun(t, server, agent.Id.String(), key.Key, "request-1", changed, http.StatusUnprocessableEntity)
	syncBody := map[string]any{"input": map[string]string{"message": "Sync replay"}, "mode": "sync"}
	syncFirst := postSyncRun(t, server, agent.Id.String(), key.Key, "sync-request", syncBody, http.StatusOK)
	syncReplay := postSyncRun(t, server, agent.Id.String(), key.Key, "sync-request", syncBody, http.StatusOK)
	if syncFirst.Id != syncReplay.Id || syncReplay.Status != gen.RunStatusSucceeded {
		t.Fatalf("sync replay first=%s replay=%s", syncFirst.Id, syncReplay.Id)
	}

	var deliveries gen.WebhookDeliveryPage
	catalog.request("GET", "/v1/runs/"+first.Accepted.RunId.String()+"/webhook-deliveries", nil, http.StatusOK, &deliveries)
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != gen.WebhookDeliveryStatusDelivered || deliveries.Items[0].Attempts != 1 {
		t.Fatalf("deliveries=%+v", deliveries.Items)
	}

	receiverStatus.Store(http.StatusGone)
	failedBody := map[string]any{"input": map[string]string{"message": "Receiver gone"}, "mode": "async", "webhook_url": receiver.URL}
	failed := postAsyncRun(t, server, agent.Id.String(), key.Key, "request-gone", failedBody, http.StatusAccepted)
	select {
	case <-received:
	case <-time.After(10 * time.Second):
		t.Fatal("không nhận callback 410")
	}
	failedDelivery := waitForDeliveryStatus(t, catalog, failed.Accepted.RunId.String(), gen.WebhookDeliveryStatusFailed)
	receiverStatus.Store(http.StatusNoContent)
	var reset gen.WebhookDelivery
	catalog.request("POST", "/v1/webhook-deliveries/"+failedDelivery.Id.String()+"/retry", nil, http.StatusOK, &reset)
	if reset.Status != gen.WebhookDeliveryStatusPending || reset.Attempts != 0 {
		t.Fatalf("reset=%+v", reset)
	}
	select {
	case callback = <-received:
	case <-time.After(10 * time.Second):
		t.Fatal("không nhận callback gửi lại")
	}
	verifyWebhook(t, callback, key.WebhookSecret)
	waitForDeliveryStatus(t, catalog, failed.Accepted.RunId.String(), gen.WebhookDeliveryStatusDelivered)
}

func waitForDeliveryStatus(t *testing.T, catalog catalogHTTP, runID string, status gen.WebhookDeliveryStatus) gen.WebhookDelivery {
	t.Helper()
	for range 40 {
		var page gen.WebhookDeliveryPage
		catalog.request("GET", "/v1/runs/"+runID+"/webhook-deliveries", nil, http.StatusOK, &page)
		if len(page.Items) == 1 && page.Items[0].Status == status {
			return page.Items[0]
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("delivery của run %s không đạt trạng thái %s", runID, status)
	return gen.WebhookDelivery{}
}

func postSyncRun(t *testing.T, server *httptest.Server, agentID, apiKey, idempotencyKey string, body any, want int) gen.Run {
	t.Helper()
	raw, _ := json.Marshal(body)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/agents/"+agentID+"/runs", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-API-Key", apiKey)
	request.Header.Set("Idempotency-Key", idempotencyKey)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != want {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("status=%d want=%d body=%s", response.StatusCode, want, data)
	}
	var run gen.Run
	if json.NewDecoder(response.Body).Decode(&run) != nil {
		t.Fatal("không đọc được response sync")
	}
	return run
}

type asyncResponse struct {
	Accepted gen.RunAccepted
	Location string
}

func postAsyncRun(t *testing.T, server *httptest.Server, agentID, apiKey, idempotencyKey string, body any, want int) asyncResponse {
	t.Helper()
	raw, _ := json.Marshal(body)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/agents/"+agentID+"/runs", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-API-Key", apiKey)
	request.Header.Set("Idempotency-Key", idempotencyKey)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != want {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("status=%d want=%d body=%s", response.StatusCode, want, data)
	}
	result := asyncResponse{Location: response.Header.Get("Location")}
	if want == http.StatusAccepted && json.NewDecoder(response.Body).Decode(&result.Accepted) != nil {
		t.Fatal("không đọc được response async")
	}
	return result
}

func verifyWebhook(t *testing.T, callback receivedWebhook, secret string) {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		t.Fatal(err)
	}
	message := callback.headers.Get("webhook-id") + "." + callback.headers.Get("webhook-timestamp") + "." + string(callback.body)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	want := "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(callback.headers.Get("webhook-signature")), []byte(want)) {
		t.Fatal("chữ ký webhook không hợp lệ")
	}
}
