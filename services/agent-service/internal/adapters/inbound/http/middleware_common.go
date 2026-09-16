package http

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

func commonMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			requestID := uuid.NewString()
			w.Header().Set("X-Request-ID", requestID)
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			writer := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				if recover() != nil {
					// Panic values and stack locals can contain credentials.
					if writer.Status() == 0 {
						writeProblem(writer, 500, "Không thể xử lý yêu cầu", "internal")
					}
					log.Error("Lỗi xử lý yêu cầu", "request_id", requestID)
				}
				// Deliberately omit raw URLs, query strings, headers and bodies.
				log.Info("HTTP", "request_id", requestID, "method", r.Method, "status", writer.Status(), "duration_ms", time.Since(started).Milliseconds())
			}()
			next.ServeHTTP(writer, r)
		})
	}
}

func corsMiddleware(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		allowed[origin] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			w.Header().Add("Vary", "Origin")
			if origin != "" && !allowed[origin] {
				writeProblem(w, 403, "Địa chỉ trang đang truy cập chưa được cho phép", "forbidden")
				return
			}
			// Missing Origin is allowed for native clients; reject explicit cross-site browser requests.
			if origin == "" && r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				writeProblem(w, 403, "Địa chỉ trang đang truy cập chưa được cho phép", "forbidden")
				return
			}
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			}
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Add("Vary", "Access-Control-Request-Method")
				w.Header().Add("Vary", "Access-Control-Request-Headers")
				if origin == "" || !validPreflight(r) {
					writeProblem(w, 403, "Yêu cầu trình duyệt chưa được cho phép", "forbidden")
					return
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func validPreflight(r *http.Request) bool {
	switch r.Header.Get("Access-Control-Request-Method") {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS":
	default:
		return false
	}
	for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
		switch strings.ToLower(strings.TrimSpace(header)) {
		case "", "authorization", "content-type", "x-api-key":
		default:
			return false
		}
	}
	return true
}
