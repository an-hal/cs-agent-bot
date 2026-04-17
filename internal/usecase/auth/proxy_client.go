// Package auth implements the dashboard authentication usecase.
//
// Auth is delegated to an external service (ms-auth-proxy.up.railway.app).
// The dashboard backend only adds a whitelist gate, builds Google OAuth
// session tokens, and issues httpOnly cookies. No users / sessions /
// passwords are stored locally.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProxyLoginResponse mirrors the upstream ms-auth-proxy response we care about.
type ProxyLoginResponse struct {
	AccessToken string                 `json:"access_token"`
	Expire      string                 `json:"expire"`
	User        map[string]interface{} `json:"user"`
}

// ErrInvalidCredentials is returned when the upstream auth service rejects the credentials.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")

// ErrProxyUnavailable is returned when the upstream auth service is unreachable.
var ErrProxyUnavailable = errors.New("auth: upstream auth service unavailable")

// AuthProxyClient calls the external Sejutacita auth service.
type AuthProxyClient interface {
	Login(ctx context.Context, email, password string) (*ProxyLoginResponse, error)
}

type httpAuthProxyClient struct {
	baseURL string
	hc      *http.Client
}

// NewAuthProxyClient builds a client targeting ms-auth-proxy. baseURL must include
// the scheme (e.g. https://ms-auth-proxy.up.railway.app).
func NewAuthProxyClient(baseURL string) AuthProxyClient {
	return &httpAuthProxyClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		hc:      &http.Client{Timeout: 10 * time.Second},
	}
}

// NewAuthProxyClientWithHTTP allows tests to inject a custom *http.Client.
func NewAuthProxyClientWithHTTP(baseURL string, hc *http.Client) AuthProxyClient {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &httpAuthProxyClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		hc:      hc,
	}
}

func (c *httpAuthProxyClient) Login(ctx context.Context, email, password string) (*ProxyLoginResponse, error) {
	body, err := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	if err != nil {
		return nil, fmt.Errorf("auth proxy: marshal request: %w", err)
	}

	url := c.baseURL + "/api/v1/auth/login"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("auth proxy: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProxyUnavailable, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrInvalidCredentials
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrProxyUnavailable, resp.StatusCode, string(raw))
	}

	var out ProxyLoginResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("auth proxy: decode response: %w", err)
	}
	if out.AccessToken == "" {
		return nil, fmt.Errorf("auth proxy: empty access_token in response")
	}
	return &out, nil
}
