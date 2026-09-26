package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/alicebob/miniredis/v2"
	"github.com/feaziest/kfdesktopbe/internal/middleware"
	"github.com/redis/go-redis/v9"
)

type mockTokenVerifier struct {
	verifyFunc func(ctx context.Context, idToken string) (*auth.Token, error)
}

func (m *mockTokenVerifier) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	return m.verifyFunc(ctx, idToken)
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestFirebaseAuthMiddleware(t *testing.T) {
	t.Run("missing authorization header returns 401 ERR_UNAUTHORIZED", func(t *testing.T) {
		verifier := &mockTokenVerifier{}
		handler := middleware.FirebaseAuthMiddleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}

		var res errorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if res.Error.Code != "ERR_UNAUTHORIZED" {
			t.Errorf("expected error code ERR_UNAUTHORIZED, got %s", res.Error.Code)
		}
	})

	t.Run("invalid header format (not Bearer) returns 401 ERR_UNAUTHORIZED", func(t *testing.T) {
		verifier := &mockTokenVerifier{}
		handler := middleware.FirebaseAuthMiddleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}

		var res errorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if res.Error.Code != "ERR_UNAUTHORIZED" {
			t.Errorf("expected error code ERR_UNAUTHORIZED, got %s", res.Error.Code)
		}
	})

	t.Run("empty bearer token returns 401 ERR_UNAUTHORIZED", func(t *testing.T) {
		verifier := &mockTokenVerifier{}
		handler := middleware.FirebaseAuthMiddleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		req.Header.Set("Authorization", "Bearer    ")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("nil verifier returns 401 ERR_UNAUTHORIZED", func(t *testing.T) {
		handler := middleware.FirebaseAuthMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		req.Header.Set("Authorization", "Bearer some-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("case-insensitive bearer prefix works", func(t *testing.T) {
		verifier := &mockTokenVerifier{
			verifyFunc: func(ctx context.Context, idToken string) (*auth.Token, error) {
				if idToken == "token123" {
					return &auth.Token{UID: "uid123"}, nil
				}
				return nil, errors.New("bad token")
			},
		}
		handler := middleware.FirebaseAuthMiddleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		req.Header.Set("Authorization", "bearer token123")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("invalid token verification failure returns 401 ERR_UNAUTHORIZED", func(t *testing.T) {
		verifier := &mockTokenVerifier{
			verifyFunc: func(ctx context.Context, idToken string) (*auth.Token, error) {
				return nil, errors.New("token has expired")
			},
		}
		handler := middleware.FirebaseAuthMiddleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		req.Header.Set("Authorization", "Bearer invalid-or-expired-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401, got %d", rec.Code)
		}

		var res errorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if res.Error.Code != "ERR_UNAUTHORIZED" {
			t.Errorf("expected error code ERR_UNAUTHORIZED, got %s", res.Error.Code)
		}
	})

	t.Run("valid token injects UID into context and calls next handler", func(t *testing.T) {
		const expectedUID = "user-firebase-123"
		verifier := &mockTokenVerifier{
			verifyFunc: func(ctx context.Context, idToken string) (*auth.Token, error) {
				if idToken != "valid-mock-token" {
					return nil, errors.New("unexpected token")
				}
				return &auth.Token{UID: expectedUID}, nil
			},
		}

		var capturedUID string
		var capturedOK bool
		handler := middleware.FirebaseAuthMiddleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedUID, capturedOK = middleware.GetUID(r.Context())
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		req.Header.Set("Authorization", "Bearer valid-mock-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !capturedOK {
			t.Errorf("expected GetUID to return ok=true")
		}
		if capturedUID != expectedUID {
			t.Errorf("expected UID %s, got %s", expectedUID, capturedUID)
		}
	})
}

func TestRedisRateLimiter(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	t.Run("allows requests within limit and blocks when exceeded with 429 ERR_RATE_LIMITED", func(t *testing.T) {
		limit := 3
		window := time.Minute
		rateLimiter := middleware.RedisRateLimiter(rdb, limit, window)

		nextCalled := 0
		handler := rateLimiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled++
			w.WriteHeader(http.StatusOK)
		}))

		// Send 3 requests under limit with same IP
		for i := 1; i <= limit; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
			req.RemoteAddr = "192.168.1.100:12345"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
			}
		}

		if nextCalled != limit {
			t.Fatalf("expected nextCalled=%d, got %d", limit, nextCalled)
		}

		// 4th request must exceed rate limit
		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected status 429, got %d", rec.Code)
		}

		var res errorEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if res.Error.Code != "ERR_RATE_LIMITED" {
			t.Errorf("expected error code ERR_RATE_LIMITED, got %s", res.Error.Code)
		}
	})

	t.Run("rate limits by UID when UID is present in context", func(t *testing.T) {
		limit := 2
		window := time.Minute
		rateLimiter := middleware.RedisRateLimiter(rdb, limit, window)

		handler := rateLimiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Request for userA
		for i := 0; i < limit; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
			req = req.WithContext(middleware.WithUID(req.Context(), "user-A"))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("user-A request %d: expected 200, got %d", i, rec.Code)
			}
		}

		// userA exceeded
		reqA := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		reqA = reqA.WithContext(middleware.WithUID(reqA.Context(), "user-A"))
		recA := httptest.NewRecorder()
		handler.ServeHTTP(recA, reqA)
		if recA.Code != http.StatusTooManyRequests {
			t.Fatalf("user-A expected 429, got %d", recA.Code)
		}

		// userB should still be allowed
		reqB := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		reqB = reqB.WithContext(middleware.WithUID(reqB.Context(), "user-B"))
		recB := httptest.NewRecorder()
		handler.ServeHTTP(recB, reqB)
		if recB.Code != http.StatusOK {
			t.Fatalf("user-B expected 200, got %d", recB.Code)
		}
	})

	t.Run("nil redis client allows requests to pass through", func(t *testing.T) {
		rateLimiter := middleware.RedisRateLimiter(nil, 5, time.Minute)
		handler := rateLimiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})
}

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	handler := middleware.LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"created":true}`))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", nil)
	req = req.WithContext(middleware.WithUID(req.Context(), "user-log-test"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to decode slog JSON output: %v (raw: %s)", err, buf.String())
	}

	if logEntry["method"] != "POST" {
		t.Errorf("expected method POST, got %v", logEntry["method"])
	}
	if logEntry["path"] != "/api/v1/projects" {
		t.Errorf("expected path /api/v1/projects, got %v", logEntry["path"])
	}
	if status, ok := logEntry["status"].(float64); !ok || int(status) != 201 {
		t.Errorf("expected status 201, got %v", logEntry["status"])
	}
	if logEntry["uid"] != "user-log-test" {
		t.Errorf("expected uid user-log-test, got %v", logEntry["uid"])
	}
	if _, ok := logEntry["latency_ms"]; !ok {
		t.Errorf("expected latency_ms field in log")
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	t.Run("cancels context after specified timeout duration", func(t *testing.T) {
		timeout := 50 * time.Millisecond
		handler := middleware.TimeoutMiddleware(timeout)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-time.After(200 * time.Millisecond):
				w.WriteHeader(http.StatusOK)
			case <-r.Context().Done():
				w.WriteHeader(http.StatusGatewayTimeout)
			}
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/slow", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("expected 504 on timeout, got %d", rec.Code)
		}
	})

	t.Run("passes through without cancellation if within timeout", func(t *testing.T) {
		timeout := 100 * time.Millisecond
		handler := middleware.TimeoutMiddleware(timeout)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/fast", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})
}

func TestCORSMiddleware(t *testing.T) {
	cors := middleware.CORSMiddleware()

	t.Run("handles OPTIONS preflight request", func(t *testing.T) {
		handler := cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("next handler should not be called for OPTIONS preflight")
		}))

		req := httptest.NewRequest(http.MethodOptions, "/api/v1/projects", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
			t.Fatalf("expected 204 or 200 for OPTIONS, got %d", rec.Code)
		}

		if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin == "" {
			t.Errorf("missing Access-Control-Allow-Origin header")
		}
		if methods := rec.Header().Get("Access-Control-Allow-Methods"); methods == "" {
			t.Errorf("missing Access-Control-Allow-Methods header")
		}
		if headers := rec.Header().Get("Access-Control-Allow-Headers"); headers == "" {
			t.Errorf("missing Access-Control-Allow-Headers header")
		}
	})

	t.Run("adds CORS headers and delegates to next handler on normal requests", func(t *testing.T) {
		called := false
		handler := cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		req.Header.Set("Origin", "app://desktop")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if !called {
			t.Fatalf("expected next handler to be called")
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin == "" {
			t.Errorf("missing Access-Control-Allow-Origin header")
		}
	})

	t.Run("sets wildcard origin when Origin header is omitted", func(t *testing.T) {
		handler := cors(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if origin := rec.Header().Get("Access-Control-Allow-Origin"); origin != "*" {
			t.Errorf("expected wildcard origin *, got %s", origin)
		}
	})
}

func TestLoggingMiddleware_NilLogger(t *testing.T) {
	handler := middleware.LoggingMiddleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTimeoutMiddleware_DefaultDuration(t *testing.T) {
	handler := middleware.TimeoutMiddleware(0)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Error("expected context to have a deadline")
		}
		if time.Until(deadline) < 8*time.Second {
			t.Errorf("expected deadline ~10s in future, got %v", time.Until(deadline))
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

