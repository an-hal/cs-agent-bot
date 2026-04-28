// JWT auth via remote token introspection.
//
// The Bearer token from the Authorization header is forwarded to a remote
// auth proxy (default: ms-auth-proxy/api/v1/auth/me) which returns the
// authenticated user's identity. Validation results are cached in-memory by
// SHA-256(token) for `cacheTTL` so a hot client doesn't hit the auth proxy
// on every request — typical hit rate is 99%+ for a busy session.
//
// Trade-offs vs local JWT verify:
//   + Revocation works immediately (cache TTL = max staleness)
//   + No need to manage signing keys / JWKS locally
//   + Always-fresh email/platform claims
//   - Auth proxy availability becomes critical (mitigated by cache for short
//     blips; longer outages should bubble up)
//   - +50–200ms on cold cache; +0ms on warm cache
//
// DEV bypass (`Bearer DEV.<email>`) is preserved for local development and
// short-circuits the network call entirely.

package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/rs/zerolog"
	"golang.org/x/sync/singleflight"
)

// authMeResponse mirrors the ms-auth-proxy /auth/me payload:
//
//	{
//	  "requestId": "...",
//	  "status": "success",
//	  "message": "Authenticated",
//	  "data": {
//	    "user_id": 8,
//	    "email": "user@example.com",
//	    "platform": "hris_employer_web",
//	    "expires_at": "2026-04-28T14:51:52Z"
//	  }
//	}
type authMeResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		UserID    int    `json:"user_id"`
		Email     string `json:"email"`
		Platform  string `json:"platform"`
		ExpiresAt string `json:"expires_at"`
	} `json:"data"`
}

// JWTUser holds the authenticated user information injected into the request
// context by JWTAuthMiddleware. Roles is kept on the struct for backward
// compatibility (HasRole still works) but the introspection endpoint doesn't
// populate it — workspace-scoped permissions go through DB-backed RBAC at
// `team_member` / `role_permissions` instead.
type JWTUser struct {
	SessionID    string
	ID           string
	Email        string
	Roles        []string
	Platform     string
	NormalizedID string
}

type jwtContextKey string

const jwtUserKey jwtContextKey = "jwt_user"

// WithJWTUser stores JWTUser in context.
func WithJWTUser(ctx context.Context, user JWTUser) context.Context {
	return context.WithValue(ctx, jwtUserKey, user)
}

// GetJWTUser retrieves JWTUser from context.
func GetJWTUser(ctx context.Context) (*JWTUser, bool) {
	if ctx == nil {
		return nil, false
	}
	val := ctx.Value(jwtUserKey)
	if val == nil {
		return nil, false
	}
	user, ok := val.(JWTUser)
	if !ok {
		return nil, false
	}
	return &user, true
}

// ── DEV bypass ────────────────────────────────────────────────────

// devBypassPrefix is the sentinel that marks a dev-mode bypass token.
// Format: "Bearer DEV.<email>" — only honored when env is dev/local AND
// JWT_DEV_BYPASS_ENABLED=true. Optional roles via "X-Dev-Roles" header
// (comma-separated). See features/00-shared/13-dev-bypass.md.
const devBypassPrefix = "DEV."

func isDevEnv(env string) bool {
	return env == "development" || env == "local"
}

// tryDevBypass returns a synthesized JWTUser when the request carries a
// DEV.<email> token AND the environment + flag permit bypass. Returns nil
// otherwise so the caller falls through to real validation.
func tryDevBypass(token, env string, enabled bool, devRolesHeader string, logger zerolog.Logger) *JWTUser {
	if !enabled || !isDevEnv(env) {
		return nil
	}
	if !strings.HasPrefix(token, devBypassPrefix) {
		return nil
	}
	email := strings.TrimSpace(strings.TrimPrefix(token, devBypassPrefix))
	if email == "" || !strings.Contains(email, "@") {
		return nil
	}

	roles := []string{"admin"}
	if devRolesHeader = strings.TrimSpace(devRolesHeader); devRolesHeader != "" {
		roles = roles[:0]
		for _, r := range strings.Split(devRolesHeader, ",") {
			if r = strings.TrimSpace(r); r != "" {
				roles = append(roles, r)
			}
		}
	}

	logger.Warn().
		Str("email", email).
		Strs("roles", roles).
		Str("env", env).
		Msg("JWT: DEV BYPASS active — remote validation skipped")

	return &JWTUser{
		SessionID:    "dev-session",
		ID:           "dev-user",
		Email:        email,
		Roles:        roles,
		Platform:     "dev",
		NormalizedID: "dev-user",
	}
}

// ── Cache ─────────────────────────────────────────────────────────

// cacheTTL is short on purpose: it bounds how stale a revoked token can
// remain valid in our process. 60s is a good default — long enough to
// shave 99%+ of hot-path requests, short enough that a "log out everywhere"
// action propagates within a minute.
const cacheTTL = 60 * time.Second

// negativeCacheTTL is shorter. We cache 401/403 outcomes briefly so a flood
// of replays from a misbehaving client doesn't hammer the auth proxy.
const negativeCacheTTL = 10 * time.Second

type cacheEntry struct {
	user      JWTUser
	authOK    bool   // false = negative cache (401/403)
	errorMsg  string // populated when authOK=false
	expiresAt time.Time
}

type tokenCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func newTokenCache() *tokenCache {
	return &tokenCache{entries: make(map[string]cacheEntry)}
}

// hashToken returns a SHA-256 hex digest. We never key the cache on the raw
// token to avoid keeping plaintext credentials in process memory longer than
// necessary.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// get returns the cached entry if present and not yet expired. Concurrent
// reads are safe; the rare stale-entry race window is bounded by cacheTTL.
func (c *tokenCache) get(key string) (cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok {
		return cacheEntry{}, false
	}
	if time.Now().After(entry.expiresAt) {
		return cacheEntry{}, false
	}
	return entry, true
}

func (c *tokenCache) set(key string, entry cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry
}

// reapLoop periodically deletes expired entries so cache memory doesn't
// grow unbounded for tokens we never see again. Runs once per cacheTTL.
func (c *tokenCache) reapLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(cacheTTL)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			c.mu.Lock()
			for k, e := range c.entries {
				if now.After(e.expiresAt) {
					delete(c.entries, k)
				}
			}
			c.mu.Unlock()
		}
	}
}

// ── Middleware ────────────────────────────────────────────────────

// JWTAuthMiddleware validates JWT tokens via remote introspection at
// validateURL (default: ms-auth-proxy /api/v1/auth/me). Successful
// validations are cached in-memory for cacheTTL. Concurrent cache misses
// for the same token are coalesced (singleflight) so the auth proxy
// receives a single request under thundering-herd, and a circuit breaker
// fails fast when the proxy is sustained-down (avoids wasting 5s per
// request waiting for a timeout).
//
// DEV bypass tokens (`Bearer DEV.<email>`) short-circuit when env+flag permit.
func JWTAuthMiddleware(validateURL, env string, devBypassEnabled bool, logger zerolog.Logger) func(ErrorHandler) ErrorHandler {
	client := &http.Client{Timeout: 5 * time.Second}
	cache := newTokenCache()
	go cache.reapLoop(make(chan struct{})) // long-lived; intentionally not stopped

	// singleflight coalesces concurrent introspections of the same token.
	// Without this, a page that fires 20 parallel API calls on cold cache
	// triggers 20 introspection round-trips; with it, exactly 1 happens
	// and the other 19 reuse the result.
	var sfGroup singleflight.Group

	// Circuit breaker around the auth proxy. After 5 consecutive upstream
	// failures, all subsequent calls fail-fast for 30s before a single
	// probe is allowed. This prevents the entire API from grinding to a
	// halt at the 5s HTTP timeout when the auth proxy is down.
	breaker := NewCircuit(CircuitConfig{FailureThreshold: 5, Cooldown: 30 * time.Second})

	return func(next ErrorHandler) ErrorHandler {
		return func(w http.ResponseWriter, r *http.Request) error {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				logger.Warn().Msg("JWT: missing Authorization header")
				return apperror.Unauthorized("missing authorization header")
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				logger.Warn().Msg("JWT: invalid Authorization header format")
				return apperror.Unauthorized("invalid authorization header format")
			}

			// Dev-mode bypass: triple-gated (env + flag + token prefix).
			if user := tryDevBypass(token, env, devBypassEnabled, r.Header.Get("X-Dev-Roles"), logger); user != nil {
				return next(w, r.WithContext(WithJWTUser(r.Context(), *user)))
			}

			// Cache check before hitting the auth proxy.
			cacheKey := hashToken(token)
			if entry, ok := cache.get(cacheKey); ok {
				if !entry.authOK {
					return apperror.Unauthorized(entry.errorMsg)
				}
				return next(w, r.WithContext(WithJWTUser(r.Context(), entry.user)))
			}

			// Coalesce: only the first goroutine for this cacheKey calls
			// introspectToken; concurrent waiters get the same (user, err).
			res, err, _ := sfGroup.Do(cacheKey, func() (interface{}, error) {
				// Re-check the cache inside the singleflight critical section:
				// a peer that just finished may have populated it while we
				// were queued, in which case we shouldn't issue another call.
				if entry, ok := cache.get(cacheKey); ok && entry.authOK {
					return entry.user, nil
				}
				// Run introspection through the circuit breaker. The trick:
				// a token rejection (401/403 from upstream) means the upstream
				// is healthy and just disagreed about this token — it must NOT
				// count toward the breaker's failure threshold, otherwise 5
				// expired tokens would trip the breaker for everyone. Real
				// upstream failures (network errors, 5xx) DO count.
				var fetched *JWTUser
				var tokenRejection error
				ierr := breaker.Do(func() error {
					u, e := introspectToken(r.Context(), client, validateURL, token, logger)
					if e == nil {
						fetched = u
						return nil
					}
					if isTokenRejection(e) {
						tokenRejection = e
						return nil // upstream healthy → don't trip breaker
					}
					return e // network / decode / 5xx → counts as failure
				})
				if ierr != nil {
					if errors.Is(ierr, ErrCircuitOpen) {
						logger.Warn().Msg("JWT: circuit breaker open — auth proxy down, failing fast")
						return nil, apperror.Unauthorized("auth service unavailable; retry shortly")
					}
					// Real upstream failure (network/5xx/decode). The breaker
					// already counted it; we surface 401 to the client rather
					// than 500 so the FE's auth-error path triggers re-login.
					// Underlying ierr is logged inside introspectToken.
					return nil, apperror.Unauthorized("token validation failed")
				}
				if tokenRejection != nil {
					cache.set(cacheKey, cacheEntry{
						authOK:    false,
						errorMsg:  tokenRejection.Error(),
						expiresAt: time.Now().Add(negativeCacheTTL),
					})
					return nil, tokenRejection
				}
				cache.set(cacheKey, cacheEntry{
					user:      *fetched,
					authOK:    true,
					expiresAt: time.Now().Add(cacheTTL),
				})
				logger.Debug().
					Str("user_id", fetched.ID).
					Str("email", fetched.Email).
					Msg("JWT: authenticated (introspection)")
				return *fetched, nil
			})
			if err != nil {
				return err
			}
			user, ok := res.(JWTUser)
			if !ok {
				return apperror.InternalError(fmt.Errorf("singleflight returned unexpected type %T", res))
			}
			return next(w, r.WithContext(WithJWTUser(r.Context(), user)))
		}
	}
}

// isTokenRejection distinguishes "upstream rejected this token" (401/403 from
// auth proxy → upstream is healthy) from "upstream is broken" (network error,
// 5xx, decode failure). Only the latter trips the circuit breaker. This is
// critical: without it, 5 users with expired tokens would trip the breaker
// for everyone for 30s.
func isTokenRejection(err error) bool {
	ae, ok := err.(*apperror.AppError)
	if !ok {
		return false
	}
	return ae.HTTPStatus == http.StatusUnauthorized || ae.HTTPStatus == http.StatusForbidden
}

// introspectToken calls the auth-proxy /auth/me endpoint and decodes the
// response into a JWTUser. The error type matters: Unauthorized propagates as
// 401 to the caller; everything else (network, decode) becomes 500.
func introspectToken(ctx context.Context, client *http.Client, validateURL, token string, logger zerolog.Logger) (*JWTUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validateURL, nil)
	if err != nil {
		logger.Error().Err(err).Msg("JWT: failed to create validation request")
		return nil, apperror.InternalError(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logger.Error().Err(err).Msg("JWT: validation request failed")
		return nil, apperror.InternalError(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	// 401/403 → token rejected; upstream is healthy. Maps to Unauthorized
	// so isTokenRejection() returns true and the breaker stays Closed.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		logger.Warn().Int("status", resp.StatusCode).Str("body", string(body)).Msg("JWT: token rejected by auth proxy")
		return nil, apperror.Unauthorized("unauthorized")
	}
	// Non-200 that isn't 401/403 → upstream is broken. Maps to InternalError
	// so the breaker counts it. The auth proxy currently returns 500 for
	// malformed tokens; we treat those as upstream failures rather than
	// rejections. If that becomes too noisy, we can pattern-match the body.
	if resp.StatusCode != http.StatusOK {
		logger.Warn().Int("status", resp.StatusCode).Str("body", string(body)).Msg("JWT: validation returned non-200")
		return nil, apperror.InternalError(fmt.Errorf("auth proxy returned %d", resp.StatusCode))
	}

	var meResp authMeResponse
	if err := json.Unmarshal(body, &meResp); err != nil {
		logger.Error().Err(err).Str("body", string(body)).Msg("JWT: failed to decode validation response")
		return nil, apperror.InternalError(err)
	}
	if meResp.Status != "success" || meResp.Data.Email == "" {
		return nil, apperror.Unauthorized("invalid token (no email in response)")
	}

	return &JWTUser{
		ID:           fmt.Sprintf("%d", meResp.Data.UserID),
		Email:        meResp.Data.Email,
		Platform:     meResp.Data.Platform,
		NormalizedID: meResp.Data.Email, // auth proxy doesn't return a separate normalized id
		Roles:        nil,                // not provided by auth proxy; downstream RBAC reads team_member.role
	}, nil
}

// HasRole checks if the user has a specific role.
func (u JWTUser) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// String returns a string representation of the JWTUser.
func (u JWTUser) String() string {
	return fmt.Sprintf("JWTUser{id=%s, email=%s, roles=%v}", u.ID, u.Email, u.Roles)
}
