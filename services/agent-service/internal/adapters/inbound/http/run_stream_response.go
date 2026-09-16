package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"agent-platform/services/agent-service/internal/adapters/inbound/http/gen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

const sseHeartbeatInterval = 15 * time.Second

type runStreamResponse struct {
	ctx       context.Context
	principal domain.Principal
	command   inbound.RunCommand
	runs      inbound.RunUseCase
}

func (r *runStreamResponse) VisitStreamRunResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	writer := newSSEWriter(w)
	streamCtx, stopStream := context.WithCancel(r.ctx)
	var heartbeatsDone chan struct{}
	started := false
	sink := inbound.RunEventSinkFunc(func(ctx context.Context, event domain.RunEvent) error {
		if err := writer.Emit(ctx, event); err != nil {
			return err
		}
		if !started {
			started = true
			heartbeatsDone = make(chan struct{})
			go func() {
				defer close(heartbeatsDone)
				runHeartbeats(streamCtx, writer, stopStream)
			}()
		}
		return nil
	})
	_, err := r.runs.RunStream(streamCtx, r.principal, r.command, sink)
	stopStream()
	if heartbeatsDone != nil {
		<-heartbeatsDone
	}
	if err != nil && !started {
		return err
	}
	if err != nil {
		slog.Error("Luồng xử lý kết thúc trước khi tạo được kết quả", "error", err)
	}
	return nil
}

func runHeartbeats(ctx context.Context, writer *sseWriter, cancel context.CancelFunc) {
	ticker := time.NewTicker(sseHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if writer.heartbeat(ctx) != nil {
				cancel()
				return
			}
		}
	}
}

var _ gen.StreamRunResponseObject = (*runStreamResponse)(nil)
