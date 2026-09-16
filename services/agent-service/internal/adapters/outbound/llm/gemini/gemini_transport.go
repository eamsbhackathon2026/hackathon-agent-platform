package gemini

import (
	"context"
	"errors"
	"io"
	"net/http"

	"agent-platform/services/agent-service/internal/core/domain"
)

// The SDK logs scanner/Close errors internally. Sanitize only body errors before
// they reach it, retaining the injected egress transport and context cancellation.
type safeBodyTransport struct{ base http.RoundTripper }

func (t safeBodyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	response.Body = safeBody{ReadCloser: response.Body, ctx: request.Context()}
	return response, nil
}

type safeBody struct {
	io.ReadCloser
	ctx context.Context
}

func (b safeBody) Read(buffer []byte) (int, error) {
	n, err := b.ReadCloser.Read(buffer)
	return n, b.cleanError(err)
}

func (b safeBody) Close() error { return b.cleanError(b.ReadCloser.Close()) }

func (b safeBody) cleanError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) {
		return io.EOF
	}
	if b.ctx.Err() != nil {
		return b.ctx.Err()
	}
	return domain.ErrProviderUnreachable
}
