package worker

import (
	"context"
	"log/slog"
	"time"

	"agent-platform/services/agent-service/internal/core/ports/inbound"
)

// WebhookWorker polls the callback outbox independently from run execution.
type WebhookWorker struct {
	Dispatcher inbound.WebhookDispatcher
	Logger     *slog.Logger
	Poll       time.Duration
}

// Run dispatches due callback batches until shutdown.
func (w *WebhookWorker) Run(ctx context.Context) {
	if w.Poll <= 0 {
		w.Poll = 2 * time.Second
	}
	for {
		_, err := w.Dispatcher.DispatchDue(ctx)
		if err != nil && ctx.Err() == nil && w.Logger != nil {
			w.Logger.Error("Không thể gửi kết quả nền", "error", err)
		}
		if !wait(ctx, w.Poll) {
			return
		}
	}
}
