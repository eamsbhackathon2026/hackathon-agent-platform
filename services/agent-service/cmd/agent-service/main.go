// Command agent-service starts the Agent Platform HTTP service.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"agent-platform/services/agent-service/internal/config"
	"agent-platform/services/agent-service/internal/platform/logger"
)

func main() {
	slog.SetDefault(logger.New(os.Stderr, "info"))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		slog.Error("Không thể chạy dịch vụ", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(os.Stdout, cfg.LogLevel)
	slog.SetDefault(log)
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("không thể cấu hình kết nối Postgres; kiểm tra DATABASE_URL")
	}
	defer pool.Close()
	app, err := wireApplicationRuntime(cfg, pool, log)
	if err != nil {
		return errors.New("không thể khởi tạo dịch vụ; kiểm tra cấu hình và cơ sở dữ liệu")
	}
	server := &http.Server{
		Handler:           app.handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
		// Streams set their write deadline per event rather than per response.
		WriteTimeout: 0,
	}
	listener, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("không thể mở cổng HTTP: %w", err)
	}
	log.Info("Dịch vụ đã khởi động", "address", listener.Addr().String(), "environment", cfg.AppEnv)
	backgroundCtx, stopBackground := context.WithCancel(ctx)
	backgroundDone := make(chan struct{})
	go func() { defer close(backgroundDone); app.runBackground(backgroundCtx) }()
	serveErr := serve(ctx, server, listener)
	stopBackground()
	select {
	case <-backgroundDone:
	case <-time.After(30 * time.Second):
		return errors.Join(serveErr, errors.New("worker không dừng trong 30 giây"))
	}
	return serveErr
}
