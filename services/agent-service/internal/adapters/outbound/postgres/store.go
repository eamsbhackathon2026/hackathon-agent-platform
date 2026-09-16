package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"agent-platform/services/agent-service/internal/adapters/outbound/postgres/sqlcgen"
	"agent-platform/services/agent-service/internal/core/domain"
	"agent-platform/services/agent-service/internal/core/ports/outbound"
)

// Store implements identity persistence over a pool and context-bound transactions.
type Store struct{ pool *pgxpool.Pool }

// NewStore binds identity repositories to a shared PostgreSQL pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

type txKey struct{ store *Store }

func (s *Store) queries(ctx context.Context) *sqlcgen.Queries {
	if tx, ok := ctx.Value(txKey{s}).(pgx.Tx); ok {
		return sqlcgen.New(tx)
	}
	return sqlcgen.New(s.pool)
}

// WithinTx commits successful callbacks and rolls back errors or panics.
func (s *Store) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := ctx.Value(txKey{s}).(pgx.Tx); ok {
		return fn(ctx)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if err = fn(context.WithValue(ctx, txKey{s}, tx)); err != nil {
		return err
	}
	return mapError(tx.Commit(ctx))
}
func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		switch pgerr.Code {
		case "23001", "23505":
			return domain.ErrConflict
		case "23503":
			return domain.ErrNotFound
		case "23514", "23502":
			return domain.ErrValidation
		}
	}
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	return nil
}
func affected(n int64, err error) error {
	if err != nil {
		return mapError(err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var (
	_ outbound.TxManager = (*Store)(nil)

	_ outbound.UserRepository             = (*Store)(nil)
	_ outbound.RefreshTokenRepository     = (*Store)(nil)
	_ outbound.APIKeyRepository           = (*Store)(nil)
	_ outbound.SessionRepository          = (*Store)(nil)
	_ outbound.MessageRepository          = (*Store)(nil)
	_ outbound.RunRepository              = (*Store)(nil)
	_ outbound.SpanRepository             = (*Store)(nil)
	_ outbound.RunQueue                   = (*Store)(nil)
	_ outbound.IdempotencyStore           = (*Store)(nil)
	_ outbound.WebhookDeliveryRepository  = (*Store)(nil)
	_ outbound.ToolRepository             = (*Store)(nil)
	_ outbound.APIConnectionRepository    = (*Store)(nil)
	_ outbound.MCPServerRepository        = (*Store)(nil)
	_ outbound.AgentToolBindingRepository = (*Store)(nil)
)
