package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GoogleTokenInfo is the subset of Google's tokeninfo response we use.
type GoogleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	Aud           string `json:"aud"`
	EmailVerified string `json:"email_verified"` // Google returns this as a string ("true"/"false")
}

// ErrGoogleInvalidToken is returned when the supplied Google credential is invalid.
var ErrGoogleInvalidToken = errors.New("auth: invalid google credential")

// GoogleTokenVerifier verifies a Google ID token by calling Google's tokeninfo endpoint.
type GoogleTokenVerifier interface {
	Verify(ctx context.Context, credential string) (*GoogleTokenInfo, error)
}

type googleTokenVerifier struct {
	clientID string
	hc       *http.Client
	baseURL  string
}

// NewGoogleTokenVerifier returns a verifier that calls oauth2.googleapis.com.
// clientID must be the Google OAuth Client ID configured for this dashboard.
func NewGoogleTokenVerifier(clientID string) GoogleTokenVerifier {
	return &googleTokenVerifier{
		clientID: clientID,
		hc:       &http.Client{Timeout: 10 * time.Second},
		baseURL:  "https://oauth2.googleapis.com/tokeninfo",
	}
}

// NewGoogleTokenVerifierWithHTTP allows injecting a custom http client and base URL for tests.
func NewGoogleTokenVerifierWithHTTP(clientID, baseURL string, hc *http.Client) GoogleTokenVerifier {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &googleTokenVerifier{clientID: clientID, hc: hc, baseURL: baseURL}
}

func (v *googleTokenVerifier) Verify(ctx context.Context, credential string) (*GoogleTokenInfo, error) {
	if strings.TrimSpace(credential) == "" {
		return nil, ErrGoogleInvalidToken
	}
	if v.clientID == "" {
		return nil, fmt.Errorf("auth: google client id not configured")
	}

	q := url.Values{}
	q.Set("id_token", credential)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("auth: build google verify request: %w", err)
	}

	resp, err := v.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: google verify request: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, ErrGoogleInvalidToken
	}

	var info GoogleTokenInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return nil, fmt.Errorf("auth: decode google tokeninfo: %w", err)
	}
	if info.Aud != v.clientID {
		return nil, ErrGoogleInvalidToken
	}
	if !strings.EqualFold(info.EmailVerified, "true") {
		return nil, ErrGoogleInvalidToken
	}
	if info.Sub == "" || info.Email == "" {
		return nil, ErrGoogleInvalidToken
	}
	return &info, nil
}
