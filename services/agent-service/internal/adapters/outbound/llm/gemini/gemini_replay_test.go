package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestReplayPreservesEmptyTextOneofAndSignaturePositions(t *testing.T) {
	requests := 0
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			writeSSE(w, `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"weather","args":{}},"thoughtSignature":"Y2FsbA=="}]}}]}`+"\n"+
				`{"candidates":[{"content":{"parts":[{"text":"","thoughtSignature":"c2lnbmVkLWVtcHR5"},{"text":""}]},"finishReason":"STOP"}]}`)
			return
		}
		// Decode the actual wire request; SDK Part.Text cannot distinguish absent
		// from explicitly empty, so comparing typed Parts would miss this regression.
		var body struct {
			Contents []struct {
				Parts []map[string]json.RawMessage `json:"parts"`
			} `json:"contents"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Contents) != 3 {
			t.Error("invalid replay request")
			w.WriteHeader(400)
			return
		}
		parts := body.Contents[1].Parts
		if len(parts) != 3 || string(parts[0]["thoughtSignature"]) != `"Y2FsbA=="` || len(parts[0]["functionCall"]) == 0 {
			t.Error("tool part or ordering changed")
			w.WriteHeader(400)
			return
		}
		if _, exists := parts[0]["text"]; exists {
			t.Error("text added to a function call part")
		}
		if string(parts[1]["text"]) != `""` || string(parts[1]["thoughtSignature"]) != `"c2lnbmVkLWVtcHR5"` || string(parts[2]["text"]) != `""` {
			t.Error("empty text oneof or signature position lost")
			w.WriteHeader(400)
			return
		}
		if _, exists := parts[2]["thoughtSignature"]; exists {
			t.Error("signature moved to unsigned part")
		}
		writeSSE(w, `{"candidates":[{"content":{"parts":[{"text":"Done"}]},"finishReason":"STOP"}]}`)
	})
	req := request()
	first, err := client.Stream(context.Background(), req, nil)
	if err != nil || len(first.ToolCalls) != 1 {
		t.Fatal("initial stream failed", err)
	}
	call := first.ToolCalls[0]
	req.Messages = append(req.Messages,
		domain.ChatMessage{Role: "assistant", ToolCalls: first.ToolCalls, ProviderMeta: first.ProviderMeta},
		domain.ChatMessage{Role: "tool", ToolResults: []domain.ToolResult{{CallID: call.ID, Name: call.Name, Content: "29"}}},
	)
	if _, err := client.Stream(context.Background(), req, nil); err != nil || requests != 2 {
		t.Fatal("replay rejected", err)
	}
}
