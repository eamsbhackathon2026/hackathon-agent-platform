// Package webhook signs and sends guarded Standard Webhooks callbacks.
package webhook

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// HTTPSender sends callbacks through a preconfigured guarded client.
type HTTPSender struct{ client *http.Client }

// NewHTTPSender binds a guarded HTTP client.
func NewHTTPSender(client *http.Client) *HTTPSender { return &HTTPSender{client: client} }

// Send posts JSON without following redirects or retaining response bodies.
func (s *HTTPSender) Send(ctx context.Context, url string, headers map[string]string, body []byte) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := s.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.CopyN(io.Discard, response.Body, 1025)
	return response.StatusCode, nil
}

var _ outbound.WebhookSender = (*HTTPSender)(nil)
