//go:build integration

package http_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	httpadapter "agent-platform/services/agent-service/internal/adapters/inbound/http"
)

func TestReadinessAgainstPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		t.Fatal("cần TEST_DATABASE_URL hoặc DATABASE_URL; chạy make up trước")
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatal("không tạo được kết nối kiểm thử Postgres")
	}
	defer pool.Close()
	response := httptest.NewRecorder()
	newSystemRouter(t, httpadapter.NewSystemHandler(pool.Ping)).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("Postgres chưa sẵn sàng, /readyz=%d; kiểm tra make up và DATABASE_URL", response.Code)
	}
}
