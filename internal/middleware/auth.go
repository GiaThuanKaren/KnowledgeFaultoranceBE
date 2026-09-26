package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
)

type contextKey string

const uidContextKey contextKey = "uid"

// WithUID injects a user ID into the context.
func WithUID(ctx context.Context, uid string) context.Context {
	return context.WithValue(ctx, uidContextKey, uid)
}

// GetUID retrieves the authenticated user ID from context.
func GetUID(ctx context.Context) (string, bool) {
	uid, ok := ctx.Value(uidContextKey).(string)
	return uid, ok && uid != ""
}

// TokenVerifier defines the interface for verifying Firebase ID tokens.
// Satisfied by *auth.Client from firebase.google.com/go/v4/auth.
type TokenVerifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error)
}

// DevTokenVerifier provides a development fallback when Firebase credentials are not provided.
type DevTokenVerifier struct{}

// NewDevTokenVerifier creates a new DevTokenVerifier.
func NewDevTokenVerifier() *DevTokenVerifier {
	return &DevTokenVerifier{}
}

// VerifyIDToken verifies tokens in development mode.
// Supports tokens like "mock-token:<uid>" or defaults to UID "dev-user".
func (d *DevTokenVerifier) VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error) {
	if strings.TrimSpace(idToken) == "" {
		return nil, errors.New("empty id token")
	}
	uid := "dev-user"
	if strings.HasPrefix(idToken, "mock-token:") {
		uid = strings.TrimPrefix(idToken, "mock-token:")
	}
	return &auth.Token{UID: uid}, nil
}


type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: errorDetail{
			Code:    code,
			Message: message,
		},
	})
}

// FirebaseAuthMiddleware verifies Firebase ID tokens from the Authorization header
// and injects the UID into the request context.
func FirebaseAuthMiddleware(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Invalid authorization header format")
				return
			}

			tokenString := strings.TrimSpace(parts[1])
			if verifier == nil {
				writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Token verifier not configured")
				return
			}

			token, err := verifier.VerifyIDToken(r.Context(), tokenString)
			if err != nil || token == nil || token.UID == "" {
				writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Invalid or expired token")
				return
			}

			ctx := WithUID(r.Context(), token.UID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
