package gemini

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"agent-platform/services/agent-service/internal/core/domain"
)

func TestHTTPErrorMappingAndNoRetries(t *testing.T) {
	for _, tc := range []struct {
		status                 int
		streamError, listError error
	}{
		{400, domain.ErrProviderBadRequest, domain.ErrProviderBadRequest},
		{401, domain.ErrProviderAuth, domain.ErrProviderAuth},
		{403, domain.ErrProviderAuth, domain.ErrProviderAuth},
		{404, domain.ErrModelNotFound, domain.ErrModelsUnsupported},
		{405, domain.ErrProviderBadRequest, domain.ErrModelsUnsupported},
		{422, domain.ErrProviderBadRequest, domain.ErrProviderBadRequest},
		{429, domain.ErrProviderRateLimited, domain.ErrProviderRateLimited},
		{500, domain.ErrProviderUnreachable, domain.ErrProviderUnreachable},
		{501, domain.ErrProviderUnreachable, domain.ErrModelsUnsupported},
	} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			count := 0
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				count++
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprintf(w, `{"error":{"code":%d,"message":"fixture-secret-never-return"}}`, tc.status)
			})
			_, err := client.Stream(context.Background(), request(), nil)
			if !errors.Is(err, tc.streamError) || count != 1 {
				t.Fatalf("stream error %v requests %d", err, count)
			}
			_, err = client.ListModels(context.Background())
			if !errors.Is(err, tc.listError) || count != 2 {
				t.Fatalf("list error %v requests %d", err, count)
			}
		})
	}
}

func TestCancellationAndPartialStreamError(t *testing.T) {
	t.Run("invalid_shape", func(t *testing.T) {
		client := testClient(t, func(w http.ResponseWriter, _ *http.Request) { writeSSE(w, `{"candidates":"private-invalid-value"}`) })
		_, err := client.Stream(context.Background(), request(), nil)
		if err != domain.ErrProviderBadRequest {
			t.Fatal(err)
		}
	})
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			writeSSE(w, `{"candidates":[{"content":{"parts":[{"text":"partial"}]}}]}`)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		})
		result, err := client.Stream(ctx, request(), func(domain.LLMDelta) { cancel() })
		if !errors.Is(err, context.Canceled) || result.Text != "" || len(result.ToolCalls) != 0 {
			t.Fatalf("result %v %v", result, err)
		}
	})
	t.Run("malformed", func(t *testing.T) {
		client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
			writeSSE(w, "{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"partial\"}]}}]}\n{bad fixture-secret}")
		})
		result, err := client.Stream(context.Background(), request(), nil)
		if err != domain.ErrProviderBadRequest || result.Text != "" || len(result.ToolCalls) != 0 {
			t.Fatalf("result %v %v", result, err)
		}
	})
	t.Run("malformed_arguments", func(t *testing.T) {
		client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
			writeSSE(w, `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"weather","args":"bad-secret"}}]}}]}`)
		})
		_, err := client.Stream(context.Background(), request(), nil)
		if err != domain.ErrProviderBadRequest {
			t.Fatal(err)
		}
	})
}

type brokenBody struct{ io.Reader }

func (brokenBody) Read([]byte) (int, error) { return 0, errors.New("private-token-in-read-error") }
func (brokenBody) Close() error             { return errors.New("private-token-in-close-error") }

func TestSDKBodyErrorsAreSanitized(t *testing.T) {
	body := safeBody{ReadCloser: brokenBody{}, ctx: context.Background()}
	_, err := body.Read(make([]byte, 10))
	if err != domain.ErrProviderUnreachable || body.Close() != domain.ErrProviderUnreachable {
		t.Fatal("body errors must be sanitized")
	}
	if strings.Contains(err.Error(), "private-token") {
		t.Fatal("secret exposed")
	}
}

func TestInvalidAPIKeyWithBadRequestStatus(t *testing.T) {
	for _, tc := range []struct {
		name, detail string
		want         error
	}{
		{"invalid_key", `{"@type":"type.googleapis.com/google.rpc.ErrorInfo","domain":"googleapis.com","reason":"API_KEY_INVALID"}`, domain.ErrProviderAuth},
		{"bad_payload", `{"@type":"type.googleapis.com/google.rpc.ErrorInfo","domain":"googleapis.com","reason":"INVALID_ARGUMENT"}`, domain.ErrProviderBadRequest},
		{"message_only", `{}`, domain.ErrProviderBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"error":{"code":400,"status":"INVALID_ARGUMENT","message":"API key not valid. private diagnostic","details":[%s]}}`, tc.detail)
			})
			if _, err := client.ListModels(context.Background()); err != tc.want {
				t.Fatal("list mapping:", err)
			}
			if _, err := client.Stream(context.Background(), request(), nil); err != tc.want {
				t.Fatal("stream mapping:", err)
			}
			if calls != 2 {
				t.Fatal("invalid key caused retries")
			}
		})
	}
}
