// Command migrate applies the embedded database migrations and exits.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"agent-platform/services/agent-service/internal/platform/logger"
	"agent-platform/services/agent-service/migrations"
)

func main() {
	log := logger.New(os.Stdout, os.Getenv("LOG_LEVEL"))
	slog.SetDefault(log)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, log); err != nil {
		log.Error("Không thể áp dụng migration", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL là bắt buộc")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return errors.New("không thể mở kết nối Postgres; kiểm tra DATABASE_URL")
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Warn("Không đóng được kết nối migration", "error", err)
		}
	}()
	// The database may still be accepting connections a moment after the container starts.
	if err := waitForDatabase(ctx, db.PingContext); err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		return errors.New("không thể đọc danh sách migration")
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return err
	}
	for _, result := range results {
		log.Info("Đã áp dụng migration", "version", result.Source.Version, "name", result.Source.Path)
	}
	log.Info("Migration hoàn tất", "applied", len(results))
	return nil
}

func waitForDatabase(ctx context.Context, ping func(context.Context) error) error {
	const attempts = 30
	for range attempts {
		if err := ping(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return errors.New("không kết nối được Postgres sau 60 giây")
}
