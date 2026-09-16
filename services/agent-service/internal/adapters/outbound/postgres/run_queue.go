package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// Enqueue inserts a run job in the caller's transaction.
func (s *Store) Enqueue(ctx context.Context, job domain.RunJob) error {
	return mapError(s.queries(ctx).EnqueueRun(ctx, sqlcgen.EnqueueRunParams{RunID: dbID(job.RunID), AvailableAt: dbTime(job.AvailableAt), CreatedAt: dbTime(job.AvailableAt)}))
}

// Claim leases due jobs without allowing concurrent ownership.
func (s *Store) Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]domain.RunJob, error) {
	if limit < 1 {
		limit = 1
	} else if limit > 100 {
		limit = 100
	}
	rows, err := s.queries(ctx).ClaimRunJobs(ctx, sqlcgen.ClaimRunJobsParams{WorkerID: pgtype.Text{String: workerID, Valid: true}, LeaseMilliseconds: lease.Milliseconds(), JobLimit: int32(limit)})
	if err != nil {
		return nil, mapError(err)
	}
	items := make([]domain.RunJob, 0, len(rows))
	for _, row := range rows {
		items = append(items, domain.RunJob{RunID: uuid.UUID(row.RunID.Bytes), Attempts: int(row.Attempts), AvailableAt: row.AvailableAt.Time.UTC()})
	}
	return items, nil
}

// Extend renews a lease still owned by this worker.
func (s *Store) Extend(ctx context.Context, runID uuid.UUID, workerID string, lease time.Duration) error {
	return affected(s.queries(ctx).ExtendRunJob(ctx, sqlcgen.ExtendRunJobParams{RunID: dbID(runID), WorkerID: pgtype.Text{String: workerID, Valid: true}, LeaseMilliseconds: lease.Milliseconds()}))
}

// Complete removes a terminal job still owned by this worker.
func (s *Store) Complete(ctx context.Context, runID uuid.UUID, workerID string) error {
	return affected(s.queries(ctx).CompleteRunJob(ctx, sqlcgen.CompleteRunJobParams{RunID: dbID(runID), LockedBy: pgtype.Text{String: workerID, Valid: true}}))
}

var _ outbound.RunQueue = (*Store)(nil)
