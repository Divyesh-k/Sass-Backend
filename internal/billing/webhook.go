package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidSignature = errors.New("billing: webhook signature verification failed")
var ErrTimestampTooOld = errors.New("billing: webhook timestamp outside tolerance window")

// tolerance matches Stripe's own default replay-attack window.
const tolerance = 5 * time.Minute

// VerifyWebhookSignature implements Stripe's documented signature scheme
// by hand (HMAC-SHA256 over "timestamp.payload") so the service doesn't
// need the Stripe SDK just for this one check. This is the same
// constant-time-compare + timestamp-tolerance pattern used to protect
// every webhook receiver in this stack.
func VerifyWebhookSignature(payload []byte, sigHeader, secret string) error {
	timestamp, signatures, err := parseSigHeader(sigHeader)
	if err != nil {
		return err
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrInvalidSignature
	}
	if time.Since(time.Unix(ts, 0)) > tolerance {
		return ErrTimestampTooOld
	}

	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expected := hex.EncodeToString(mac.Sum(nil))

	for _, sig := range signatures {
		if subtle.ConstantTimeCompare([]byte(sig), []byte(expected)) == 1 {
			return nil
		}
	}
	return ErrInvalidSignature
}

// parseSigHeader parses Stripe's "t=169...,v1=abcd...,v1=efgh..." header
// format into a timestamp and the list of v1 signatures to check against.
func parseSigHeader(header string) (timestamp string, signatures []string, err error) {
	parts := strings.Split(header, ",")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}
	if timestamp == "" || len(signatures) == 0 {
		return "", nil, fmt.Errorf("%w: malformed Stripe-Signature header", ErrInvalidSignature)
	}
	return timestamp, signatures, nil
}
