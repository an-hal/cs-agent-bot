package middleware

import (
	"io"
	"testing"

	"github.com/rs/zerolog"
)

func discardLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}

func TestTryDevBypass_DisabledByFlag(t *testing.T) {
	u := tryDevBypass("DEV.user@example.com", "development", false, "", discardLogger())
	if u != nil {
		t.Fatal("expected nil when flag disabled")
	}
}

func TestTryDevBypass_DisabledByProdEnv(t *testing.T) {
	u := tryDevBypass("DEV.user@example.com", "production", true, "", discardLogger())
	if u != nil {
		t.Fatal("expected nil when env=production even with flag enabled")
	}
}

func TestTryDevBypass_DisabledByStagingEnv(t *testing.T) {
	u := tryDevBypass("DEV.user@example.com", "staging", true, "", discardLogger())
	if u != nil {
		t.Fatal("expected nil when env=staging even with flag enabled")
	}
}

func TestTryDevBypass_TokenWithoutPrefix(t *testing.T) {
	u := tryDevBypass("eyJhbGc.real.jwt", "development", true, "", discardLogger())
	if u != nil {
		t.Fatal("expected nil when token lacks DEV. prefix")
	}
}

func TestTryDevBypass_EmptyEmail(t *testing.T) {
	u := tryDevBypass("DEV.", "development", true, "", discardLogger())
	if u != nil {
		t.Fatal("expected nil when email is empty")
	}
}

func TestTryDevBypass_NoAtSign(t *testing.T) {
	u := tryDevBypass("DEV.notanemail", "development", true, "", discardLogger())
	if u != nil {
		t.Fatal("expected nil when email has no @")
	}
}

func TestTryDevBypass_DevEnvHappyPath(t *testing.T) {
	u := tryDevBypass("DEV.alice@dealls.com", "development", true, "", discardLogger())
	if u == nil {
		t.Fatal("expected synthesized user")
	}
	if u.Email != "alice@dealls.com" {
		t.Errorf("email mismatch: got %q", u.Email)
	}
	if len(u.Roles) != 1 || u.Roles[0] != "admin" {
		t.Errorf("default roles should be [admin], got %v", u.Roles)
	}
	if u.Platform != "dev" {
		t.Errorf("platform should be 'dev', got %q", u.Platform)
	}
}

func TestTryDevBypass_LocalEnv(t *testing.T) {
	u := tryDevBypass("DEV.bob@dealls.com", "local", true, "", discardLogger())
	if u == nil {
		t.Fatal("expected bypass to work on env=local")
	}
}

func TestTryDevBypass_CustomRoles(t *testing.T) {
	u := tryDevBypass("DEV.carol@dealls.com", "development", true, "super-admin, viewer ,editor", discardLogger())
	if u == nil {
		t.Fatal("expected synthesized user")
	}
	want := []string{"super-admin", "viewer", "editor"}
	if len(u.Roles) != len(want) {
		t.Fatalf("roles length: got %v want %v", u.Roles, want)
	}
	for i, r := range want {
		if u.Roles[i] != r {
			t.Errorf("role[%d]: got %q want %q", i, u.Roles[i], r)
		}
	}
}

func TestTryDevBypass_EmptyTokenAfterPrefix(t *testing.T) {
	// Whitespace-only after prefix should fail validation
	u := tryDevBypass("DEV.   ", "development", true, "", discardLogger())
	if u != nil {
		t.Fatal("expected nil for whitespace-only email")
	}
}

func TestIsDevEnv(t *testing.T) {
	cases := map[string]bool{
		"development": true,
		"local":       true,
		"production":  false,
		"staging":     false,
		"prod":        false,
		"":            false,
		"DEV":         false, // case-sensitive
	}
	for env, want := range cases {
		if got := isDevEnv(env); got != want {
			t.Errorf("isDevEnv(%q) = %v, want %v", env, got, want)
		}
	}
}
