package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
)

func TestSignAndVerifyGoogleSession_Roundtrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		secret   string
		googleID string
	}{
		{name: "basic", secret: "topsecret", googleID: "google-123"},
		{name: "long secret", secret: strings.Repeat("a", 64), googleID: "google-xyz"},
		{name: "uuid id", secret: "another", googleID: "550e8400-e29b-41d4-a716-446655440000"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			now := time.Now()
			tok, err := auth.SignGoogleSession(tc.secret, tc.googleID, now)
			if err != nil {
				t.Fatalf("sign failed: %v", err)
			}
			gotID, gotIssued, err := auth.VerifyGoogleSession(tc.secret, tok)
			if err != nil {
				t.Fatalf("verify failed: %v", err)
			}
			if gotID != tc.googleID {
				t.Errorf("googleID mismatch: got %q, want %q", gotID, tc.googleID)
			}
			if gotIssued.Unix() != now.Unix() {
				t.Errorf("issuedAt mismatch: got %v, want %v", gotIssued, now)
			}
		})
	}
}

func TestVerifyGoogleSession_Failures(t *testing.T) {
	t.Parallel()

	good, err := auth.SignGoogleSession("secret", "id-1", time.Now())
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}
	expired, err := auth.SignGoogleSession("secret", "id-1", time.Now().Add(-31*24*time.Hour))
	if err != nil {
		t.Fatalf("sign failed: %v", err)
	}

	cases := []struct {
		name   string
		secret string
		token  string
	}{
		{name: "tampered payload", secret: "secret", token: "google:other-id:1.deadbeefdeadbeef"},
		{name: "wrong secret", secret: "different", token: good},
		{name: "empty secret", secret: "", token: good},
		{name: "garbage", secret: "secret", token: "not-a-token"},
		{name: "empty token", secret: "secret", token: ""},
		{name: "wrong scheme", secret: "secret", token: "facebook:id:123.abcdef0123456789"},
		{name: "expired", secret: "secret", token: expired},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := auth.VerifyGoogleSession(tc.secret, tc.token); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}
