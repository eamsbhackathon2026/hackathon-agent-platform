// Package worker hosts the database-backed background loops.
package worker

import (
	"context"
	"github.com/google/uuid"
	"log/slog"
	"sync"
	"time"

	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/inbound"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// RunWorker claims jobs and keeps each lease alive while it executes.
type RunWorker struct {
	Queue                  outbound.RunQueue
	Processor              inbound.AsyncRunProcessor
	Logger                 *slog.Logger
	WorkerID               string
	Concurrency            int
	Poll, Lease, Heartbeat time.Duration
}

// Run claims until cancellation, then waits for active executions to finish.
func (w *RunWorker) Run(ctx context.Context) {
	if w.Concurrency <= 0 {
		w.Concurrency = 4
	}
	if w.Poll <= 0 {
		w.Poll = time.Second
	}
	if w.Lease <= 0 {
		w.Lease = 30 * time.Second
	}
	if w.Heartbeat <= 0 {
		w.Heartbeat = 10 * time.Second
	}
	semaphore := make(chan struct{}, w.Concurrency)
	var active sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			active.Wait()
			return
		case semaphore <- struct{}{}:
		}
		jobs, err := w.Queue.Claim(ctx, w.WorkerID, 1, w.Lease)
		if err != nil {
			<-semaphore
			w.log("Không thể nhận lần xử lý nền", err, nil)
			if !wait(ctx, w.Poll) {
				active.Wait()
				return
			}
			continue
		}
		if len(jobs) == 0 {
			<-semaphore
			if !wait(ctx, w.Poll) {
				active.Wait()
				return
			}
			continue
		}
		job := jobs[0]
		active.Add(1)
		go func() {
			defer active.Done()
			defer func() { <-semaphore }()
			w.process(ctx, job)
		}()
	}
}

func (w *RunWorker) process(parent context.Context, job domain.RunJob) {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go w.extend(ctx, cancel, done, job.RunID)
	err := w.Processor.ProcessJob(ctx, w.WorkerID, job)
	cancel()
	<-done
	if err != nil {
		w.log("Không thể xử lý lần chạy nền", err, []any{"run_id", job.RunID})
		return
	}
	completeCtx, stop := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer stop()
	if err = w.Queue.Complete(completeCtx, job.RunID, w.WorkerID); err != nil {
		w.log("Không thể hoàn tất hàng đợi", err, []any{"run_id", job.RunID})
	}
}

func (w *RunWorker) extend(ctx context.Context, cancel context.CancelFunc, done chan<- struct{}, runID uuid.UUID) {
	defer close(done)
	ticker := time.NewTicker(w.Heartbeat)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Queue.Extend(ctx, runID, w.WorkerID, w.Lease); err != nil {
				failures++
			} else {
				failures = 0
			}
			if failures >= 2 {
				cancel()
				return
			}
		}
	}
}

func (w *RunWorker) log(message string, err error, fields []any) {
	if w.Logger == nil {
		return
	}
	fields = append(fields, "error", err)
	w.Logger.Error(message, fields...)
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
