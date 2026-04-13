package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// GoogleSessionTTL is how long a Google-OAuth session cookie remains valid.
const GoogleSessionTTL = 30 * 24 * time.Hour

// ErrInvalidSessionToken is returned when a session token can't be parsed/verified.
var ErrInvalidSessionToken = errors.New("auth: invalid session token")

// ErrEmptySessionSecret is returned when SignGoogleSession is called without a
// configured secret. This must never silently produce an unsigned token.
var ErrEmptySessionSecret = errors.New("auth: empty session secret")

// SignGoogleSession returns a token of the form
//
//	google:{googleID}:{unixSeconds}.{hmacHex16}
//
// The HMAC covers the payload before the dot using SHA-256 with the supplied
// secret, truncated to 16 hex chars (matching the BFF implementation).
// Returns ErrEmptySessionSecret when secret is empty — callers MUST check.
func SignGoogleSession(secret, googleID string, issuedAt time.Time) (string, error) {
	if secret == "" {
		return "", ErrEmptySessionSecret
	}
	payload := fmt.Sprintf("google:%s:%d", googleID, issuedAt.Unix())
	sig := hmacHex16(secret, payload)
	return payload + "." + sig, nil
}

// VerifyGoogleSession parses the given token, recomputes the HMAC, and returns
// the embedded google ID + issuedAt. It also enforces GoogleSessionTTL.
func VerifyGoogleSession(secret, token string) (googleID string, issuedAt time.Time, err error) {
	if secret == "" {
		return "", time.Time{}, fmt.Errorf("auth: empty session secret")
	}
	idx := strings.LastIndex(token, ".")
	if idx <= 0 || idx == len(token)-1 {
		return "", time.Time{}, ErrInvalidSessionToken
	}
	payload, sig := token[:idx], token[idx+1:]
	parts := strings.SplitN(payload, ":", 3)
	if len(parts) != 3 || parts[0] != "google" {
		return "", time.Time{}, ErrInvalidSessionToken
	}
	expected := hmacHex16(secret, payload)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return "", time.Time{}, ErrInvalidSessionToken
	}
	tsSeconds, parseErr := strconv.ParseInt(parts[2], 10, 64)
	if parseErr != nil {
		return "", time.Time{}, ErrInvalidSessionToken
	}
	issuedAt = time.Unix(tsSeconds, 0)
	if time.Since(issuedAt) > GoogleSessionTTL {
		return "", time.Time{}, ErrInvalidSessionToken
	}
	return parts[1], issuedAt, nil
}

func hmacHex16(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}
