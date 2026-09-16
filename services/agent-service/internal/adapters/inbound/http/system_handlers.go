// Package http provides the application's HTTP adapter.
package http

import (
	"context"
	"time"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
)

// SystemHandler implements public process and dependency health checks.
type SystemHandler struct {
	ping func(context.Context) error
}

// NewSystemHandler accepts a database ping function, or nil for a standalone process.
func NewSystemHandler(ping func(context.Context) error) *SystemHandler {
	return &SystemHandler{ping: ping}
}

// GetHealth reports that the HTTP process is running.
func (*SystemHandler) GetHealth(context.Context, gen.GetHealthRequestObject) (gen.GetHealthResponseObject, error) {
	return gen.GetHealth200JSONResponse{Status: "ok"}, nil
}

// GetReady checks the configured database with a bounded timeout.
func (h *SystemHandler) GetReady(ctx context.Context, _ gen.GetReadyRequestObject) (gen.GetReadyResponseObject, error) {
	if h.ping != nil {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := h.ping(ctx); err != nil {
			return gen.GetReady503ApplicationProblemPlusJSONResponse{
				Type: "about:blank", Title: "Cơ sở dữ liệu chưa sẵn sàng", Status: 503, Code: "internal",
				Fields: []gen.FieldError{},
			}, nil
		}
	}
	return gen.GetReady200JSONResponse{Status: "ok"}, nil
}
