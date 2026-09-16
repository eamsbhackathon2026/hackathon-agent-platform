//go:build integration

// Package pgtest provisions isolated, migrated PostgreSQL schemas for integration tests.
package pgtest

import (
	"context"
	"os"
	"testing"
	"time"

	"agent-platform/services/agent-service/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	containerpg "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// NewPool provides a migrated schema isolated from every other test and application.
// TEST_DATABASE_URL reuses a server; otherwise a disposable Postgres 18 is started.
func NewPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		container, err := containerpg.Run(ctx, "postgres:18", containerpg.WithDatabase("identity_test"), containerpg.WithUsername("test"), containerpg.WithPassword("test"), containerpg.BasicWaitStrategies())
		if err != nil {
			t.Fatalf("start test Postgres: %v", err)
		}
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			if err := container.Terminate(cleanup); err != nil {
				t.Errorf("stop test Postgres: %v", err)
			}
		})
		dsn, err = container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			t.Fatal("get test Postgres connection")
		}
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal("parse test database configuration")
	}
	t.Cleanup(admin.Close)
	schema := "identity_test_" + uuid.New().String()
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatalf("create isolated schema: %v", err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanup, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("remove isolated schema: %v", err)
		}
	})
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("parse test pool configuration")
	}
	config.ConnConfig.RuntimeParams["search_path"] = quoted
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("create test pool")
	}
	t.Cleanup(pool.Close)
	db := stdlib.OpenDBFromPool(pool)
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("close migration connection: %v", err)
		}
	}()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		t.Fatalf("prepare migrations: %v", err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatalf("migrate isolated schema: %v", err)
	}
	return pool
}
