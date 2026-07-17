package auth

import "testing"

func TestGenerateOpaqueTokenIsUniqueAndHex(t *testing.T) {
	t1, err := generateOpaqueToken()
	if err != nil {
		t.Fatalf("generateOpaqueToken returned error: %v", err)
	}
	t2, err := generateOpaqueToken()
	if err != nil {
		t.Fatalf("generateOpaqueToken returned error: %v", err)
	}

	if t1 == t2 {
		t.Error("expected two generated tokens to differ")
	}
	if len(t1) != 64 { // 32 bytes hex-encoded = 64 chars
		t.Errorf("expected 64-char hex token, got length %d", len(t1))
	}
}

func TestHashTokenIsDeterministic(t *testing.T) {
	token := "some-opaque-token-value"
	if hashToken(token) != hashToken(token) {
		t.Error("expected hashToken to be deterministic for the same input")
	}
	if hashToken(token) == hashToken(token+"x") {
		t.Error("expected different inputs to produce different hashes")
	}
}
