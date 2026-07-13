package auth

import (
	"testing"
	"time"
)

func TestIssueAndParseAccessToken(t *testing.T) {
	tm := NewTokenManager("test-secret-key-at-least-32-bytes", time.Minute)

	token, expiresIn, err := tm.IssueAccessToken("user-123", "test@example.com")
	if err != nil {
		t.Fatalf("IssueAccessToken returned error: %v", err)
	}
	if expiresIn != 60 {
		t.Errorf("expected expiresIn=60, got %d", expiresIn)
	}

	claims, err := tm.Parse(token)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected UserID=user-123, got %s", claims.UserID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("expected Email=test@example.com, got %s", claims.Email)
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	tm := NewTokenManager("test-secret-key-at-least-32-bytes", -time.Minute)

	token, _, err := tm.IssueAccessToken("user-123", "test@example.com")
	if err != nil {
		t.Fatalf("IssueAccessToken returned error: %v", err)
	}

	if _, err := tm.Parse(token); err == nil {
		t.Error("expected expired token to fail parsing")
	}
}

func TestParseRejectsTokenSignedWithDifferentSecret(t *testing.T) {
	tm1 := NewTokenManager("secret-one-at-least-32-bytes-long", time.Minute)
	tm2 := NewTokenManager("secret-two-at-least-32-bytes-long", time.Minute)

	token, _, _ := tm1.IssueAccessToken("user-123", "test@example.com")

	if _, err := tm2.Parse(token); err == nil {
		t.Error("expected token signed with a different secret to fail parsing")
	}
}

func TestParseRejectsGarbageToken(t *testing.T) {
	tm := NewTokenManager("test-secret-key-at-least-32-bytes", time.Minute)

	if _, err := tm.Parse("not.a.valid.jwt"); err == nil {
		t.Error("expected garbage input to fail parsing")
	}
}
