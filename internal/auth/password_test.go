package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	if !VerifyPassword(hash, "correct-horse-battery-staple") {
		t.Error("expected correct password to verify successfully")
	}

	if VerifyPassword(hash, "wrong-password") {
		t.Error("expected incorrect password to fail verification")
	}
}

func TestHashPasswordProducesUniqueSaltedHashes(t *testing.T) {
	h1, _ := HashPassword("same-input")
	h2, _ := HashPassword("same-input")

	if h1 == h2 {
		t.Error("expected two hashes of the same password to differ (bcrypt salts automatically)")
	}
}
