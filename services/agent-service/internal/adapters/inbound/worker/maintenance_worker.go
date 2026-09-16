package worker

import (
	"context"
	"log/slog"
	"time"

	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// MaintenanceWorker removes expired idempotency records.
type MaintenanceWorker struct {
	Store    outbound.IdempotencyStore
	Clock    outbound.Clock
	Logger   *slog.Logger
	Interval time.Duration
}

// Run cleans once on startup and then at the configured interval.
func (w *MaintenanceWorker) Run(ctx context.Context) {
	if w.Interval <= 0 {
		w.Interval = time.Hour
	}
	for {
		if err := w.Store.DeleteExpiredIdempotency(ctx, w.Clock.Now()); err != nil && ctx.Err() == nil && w.Logger != nil {
			w.Logger.Error("Không thể dọn khóa chống lặp", "error", err)
		}
		if !wait(ctx, w.Interval) {
			return
		}
	}
}
