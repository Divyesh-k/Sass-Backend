package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func signPayload(secret string, timestamp int64, payload string) string {
	signedPayload := fmt.Sprintf("%d.%s", timestamp, payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, sig)
}

func TestVerifyWebhookSignatureValid(t *testing.T) {
	secret := "whsec_test_secret"
	payload := `{"type":"checkout.session.completed"}`
	header := signPayload(secret, time.Now().Unix(), payload)

	if err := VerifyWebhookSignature([]byte(payload), header, secret); err != nil {
		t.Errorf("expected valid signature to pass, got error: %v", err)
	}
}

func TestVerifyWebhookSignatureWrongSecret(t *testing.T) {
	payload := `{"type":"checkout.session.completed"}`
	header := signPayload("whsec_correct", time.Now().Unix(), payload)

	err := VerifyWebhookSignature([]byte(payload), header, "whsec_wrong")
	if err == nil {
		t.Error("expected signature verification to fail with wrong secret")
	}
}

func TestVerifyWebhookSignatureTamperedPayload(t *testing.T) {
	secret := "whsec_test_secret"
	header := signPayload(secret, time.Now().Unix(), `{"type":"original"}`)

	err := VerifyWebhookSignature([]byte(`{"type":"tampered"}`), header, secret)
	if err == nil {
		t.Error("expected signature verification to fail for tampered payload")
	}
}

func TestVerifyWebhookSignatureExpiredTimestamp(t *testing.T) {
	secret := "whsec_test_secret"
	payload := `{"type":"checkout.session.completed"}`
	oldTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	header := signPayload(secret, oldTimestamp, payload)

	err := VerifyWebhookSignature([]byte(payload), header, secret)
	if err != ErrTimestampTooOld {
		t.Errorf("expected ErrTimestampTooOld, got %v", err)
	}
}

func TestVerifyWebhookSignatureMalformedHeader(t *testing.T) {
	err := VerifyWebhookSignature([]byte("{}"), "not-a-valid-header", "secret")
	if err == nil {
		t.Error("expected malformed header to fail verification")
	}
}
