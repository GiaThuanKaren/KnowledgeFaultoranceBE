package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriterRecorder struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriterRecorder) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriterRecorder) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// LoggingMiddleware logs HTTP requests in structured slog format with method, path,
// status, latency_ms, and authenticated user ID.
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l := logger
			if l == nil {
				l = slog.Default()
			}

			start := time.Now()
			rec := &responseWriterRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rec, r)

			latencyMs := time.Since(start).Milliseconds()
			uid, _ := GetUID(r.Context())

			l.InfoContext(r.Context(), "http_request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.statusCode),
				slog.Int64("latency_ms", latencyMs),
				slog.String("uid", uid),
			)
		})
	}
}
