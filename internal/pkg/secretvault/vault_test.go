package secretvault

import (
	"strings"
	"testing"
)

const testKey = "0123456789abcdef0123456789abcdef" // 32 raw bytes

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	v, err := New(testKey)
	if err != nil {
		t.Fatalf("new vault: %v", err)
	}
	plain := "wa-api-token-super-secret-value"
	enc, err := v.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !strings.HasPrefix(enc, envelopeVersion) {
		t.Errorf("expected envelope prefix, got %q", enc[:min(20, len(enc))])
	}
	dec, err := v.Decrypt(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if dec != plain {
		t.Errorf("round-trip mismatch: got %q, want %q", dec, plain)
	}
}

func TestDecrypt_LegacyPlaintextPassesThrough(t *testing.T) {
	v, _ := New(testKey)
	legacy := "not-encrypted-yet"
	out, err := v.Decrypt(legacy)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if out != legacy {
		t.Errorf("expected passthrough, got %q", out)
	}
}

func TestNilVault_IsPassThrough(t *testing.T) {
	var v *Vault
	enc, _ := v.Encrypt("hello")
	if enc != "hello" {
		t.Errorf("nil vault should passthrough, got %q", enc)
	}
	dec, _ := v.Decrypt("hello")
	if dec != "hello" {
		t.Errorf("nil vault should passthrough decrypt, got %q", dec)
	}
}

func TestEncryptMap_OnlySensitiveKeys(t *testing.T) {
	v, _ := New(testKey)
	m := map[string]any{
		"api_url":      "https://halo.example",
		"wa_api_token": "secret",
		"business_id":  "biz-123",
	}
	if err := v.EncryptMap(m); err != nil {
		t.Fatalf("encrypt map: %v", err)
	}
	if !strings.HasPrefix(m["wa_api_token"].(string), envelopeVersion) {
		t.Errorf("wa_api_token not encrypted")
	}
	if m["api_url"] != "https://halo.example" {
		t.Errorf("api_url should be untouched")
	}
	if m["business_id"] != "biz-123" {
		t.Errorf("business_id should be untouched (not a secret key)")
	}
}

func TestIsSensitiveKey_Coverage(t *testing.T) {
	cases := map[string]bool{
		"wa_api_token":    true,
		"api_key":         true,
		"PASSWORD":        true,
		"webhook_secret":  true,
		"business_id":     false,
		"channel_id":      false, // contains "id" not "key"; but "key" triggers — let's verify
		"primary_key":     true,  // "key" substring
		"url":             false,
	}
	for k, want := range cases {
		if got := IsSensitiveKey(k); got != want {
			t.Errorf("IsSensitiveKey(%q) = %v, want %v", k, got, want)
		}
	}
}

func TestNew_RejectsWrongLengthKey(t *testing.T) {
	_, err := New("tooshort")
	if err == nil {
		t.Error("expected error for short key")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
