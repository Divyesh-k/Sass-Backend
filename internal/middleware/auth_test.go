package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/divyeshkakadiya/saas-backend/internal/auth"
)

func TestRequireAuthRejectsMissingHeader(t *testing.T) {
	tokens := auth.NewTokenManager("test-secret-key-at-least-32-bytes", time.Minute)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	handler := RequireAuth(tokens)(next)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
	if called {
		t.Error("expected next handler not to be called without a token")
	}
}

func TestRequireAuthRejectsInvalidToken(t *testing.T) {
	tokens := auth.NewTokenManager("test-secret-key-at-least-32-bytes", time.Minute)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	handler := RequireAuth(tokens)(next)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer garbage.token.value")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthAcceptsValidTokenAndInjectsContext(t *testing.T) {
	tokens := auth.NewTokenManager("test-secret-key-at-least-32-bytes", time.Minute)
	token, _, err := tokens.IssueAccessToken("user-42", "user@example.com")
	if err != nil {
		t.Fatalf("IssueAccessToken failed: %v", err)
	}

	var gotUserID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, _ = auth.UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := RequireAuth(tokens)(next)

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if gotUserID != "user-42" {
		t.Errorf("expected injected user ID 'user-42', got %q", gotUserID)
	}
}
