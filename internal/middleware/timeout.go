package middleware

import (
	"context"
	"net/http"
	"time"
)

// TimeoutMiddleware wraps the request context with a timeout.
// If d <= 0, it defaults to 10 seconds.
func TimeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	if d <= 0 {
		d = 10 * time.Second
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
