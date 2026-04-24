// Package secretvault is a tiny AES-256-GCM envelope for sensitive config
// values stored in JSONB (e.g. workspace_integrations.config). Keys marked
// sensitive (token, secret, password, api_key, key) are encrypted on write
// and decrypted on read. Nil or empty key disables encryption cleanly —
// existing plaintext stays readable and new writes stay plaintext.
//
// Envelope format: base64("v1:" + nonce || ciphertext).
// Any stored string that does NOT decode as this envelope is assumed to be
// legacy plaintext and returned as-is on read (forward-compatible rollout).
package secretvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const envelopeVersion = "v1:"

// Vault encrypts/decrypts values with AES-256-GCM. Nil *Vault is a legal
// pass-through (safe to call without configuration).
type Vault struct {
	gcm cipher.AEAD
}

// New creates a Vault. `key` must be 32 bytes (raw AES-256 key) or a hex/
// base64 string of that length. Empty key returns nil *Vault — callers can
// still call Encrypt/Decrypt safely (they pass through).
func New(key string) (*Vault, error) {
	if key == "" {
		return nil, nil
	}
	raw, err := decodeKey(key)
	if err != nil {
		return nil, err
	}
	if len(raw) != 32 {
		return nil, errors.New("secretvault: key must be 32 bytes (AES-256)")
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{gcm: gcm}, nil
}

// Encrypt wraps plaintext into the envelope. Nil vault returns plaintext.
func (v *Vault) Encrypt(plaintext string) (string, error) {
	if v == nil || plaintext == "" {
		return plaintext, nil
	}
	nonce := make([]byte, v.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := v.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return envelopeVersion + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt unwraps envelope. Values without the envelope prefix are returned
// as-is (legacy plaintext passthrough). Nil vault also passes through.
func (v *Vault) Decrypt(stored string) (string, error) {
	if v == nil {
		return stored, nil
	}
	if !strings.HasPrefix(stored, envelopeVersion) {
		return stored, nil // legacy / not-encrypted
	}
	ct, err := base64.StdEncoding.DecodeString(stored[len(envelopeVersion):])
	if err != nil {
		return "", err
	}
	ns := v.gcm.NonceSize()
	if len(ct) < ns {
		return "", errors.New("secretvault: ciphertext too short")
	}
	nonce, body := ct[:ns], ct[ns:]
	pt, err := v.gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// IsSensitiveKey returns true when a JSON key should be encrypted. Matches
// the same rules used by workspace_integration redactor so the two stay in
// sync.
func IsSensitiveKey(k string) bool {
	lk := strings.ToLower(k)
	for _, m := range []string{"token", "secret", "password", "api_key", "key"} {
		if strings.Contains(lk, m) {
			return true
		}
	}
	return false
}

// EncryptMap walks a config map and encrypts values for sensitive keys in place.
func (v *Vault) EncryptMap(cfg map[string]any) error {
	if v == nil || cfg == nil {
		return nil
	}
	for k, val := range cfg {
		if !IsSensitiveKey(k) {
			continue
		}
		s, ok := val.(string)
		if !ok || s == "" {
			continue
		}
		enc, err := v.Encrypt(s)
		if err != nil {
			return err
		}
		cfg[k] = enc
	}
	return nil
}

// DecryptMap walks a config map and decrypts envelope-wrapped values for
// sensitive keys in place. Non-envelope values pass through untouched.
func (v *Vault) DecryptMap(cfg map[string]any) error {
	if v == nil || cfg == nil {
		return nil
	}
	for k, val := range cfg {
		if !IsSensitiveKey(k) {
			continue
		}
		s, ok := val.(string)
		if !ok || s == "" {
			continue
		}
		dec, err := v.Decrypt(s)
		if err != nil {
			return err
		}
		cfg[k] = dec
	}
	return nil
}

// decodeKey accepts hex (64 chars), base64 (44 chars), or raw bytes string.
func decodeKey(k string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(k); err == nil && len(b) == 32 {
		return b, nil
	}
	// Try raw-URL base64 (no padding).
	if b, err := base64.RawStdEncoding.DecodeString(k); err == nil && len(b) == 32 {
		return b, nil
	}
	// Try hex.
	if b, ok := tryHex(k); ok && len(b) == 32 {
		return b, nil
	}
	if len(k) == 32 {
		return []byte(k), nil
	}
	return nil, errors.New("secretvault: key format not recognized; expected 32 bytes of base64/hex/raw")
}

func tryHex(s string) ([]byte, bool) {
	if len(s)%2 != 0 {
		return nil, false
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi, ok1 := hexNibble(s[2*i])
		lo, ok2 := hexNibble(s[2*i+1])
		if !ok1 || !ok2 {
			return nil, false
		}
		out[i] = (hi << 4) | lo
	}
	return out, true
}

func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
