package middleware

import (
	"net/http"
	"strings"

	"github.com/divyeshkakadiya/saas-backend/internal/auth"
	"github.com/divyeshkakadiya/saas-backend/internal/response"
)

// RequireAuth validates the Bearer JWT on every request it wraps and
// injects the caller's user ID + email into the request context. Any
// handler behind this middleware can trust auth.UserIDFromContext.
func RequireAuth(tokens *auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				response.Error(w, http.StatusUnauthorized, "missing_token", "authorization header must be 'Bearer <token>'")
				return
			}

			raw := strings.TrimPrefix(header, "Bearer ")
			claims, err := tokens.Parse(raw)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "invalid_token", "access token is invalid or expired")
				return
			}

			ctx := auth.ContextWithUser(r.Context(), claims.UserID, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
