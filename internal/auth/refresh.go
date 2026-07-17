package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// generateOpaqueToken creates a 256-bit random token, hex-encoded. Only
// the SHA-256 hash of this value is ever persisted, so a leaked database
// cannot be used to forge sessions — the same principle applied to API
// keys and password reset tokens throughout this service.
func generateOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: generating token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
