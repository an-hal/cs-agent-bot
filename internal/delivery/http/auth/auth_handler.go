// Package auth contains HTTP handlers for the dashboard authentication endpoints.
//
// These endpoints proxy login through the external ms-auth-proxy service and add
// a whitelist gate. They issue an httpOnly session cookie. No user/session
// records are persisted by this service.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/auth"
	"github.com/rs/zerolog"
)

// CookieName is the name of the httpOnly session cookie set after login.
const CookieName = "auth_session"

// AuthHandler exposes /auth/login, /auth/google, /auth/logout, and /whitelist endpoints.
type AuthHandler struct {
	uc          auth.AuthUsecase
	loginIPLim  *auth.RateLimiter
	loginEmlLim *auth.RateLimiter
	googleLim   *auth.RateLimiter
	whitelistLim *auth.RateLimiter
	cookieSecure bool
	logger      zerolog.Logger
	tracer      tracer.Tracer
}

// NewAuthHandler builds an AuthHandler with rate limits matching the spec:
//   - POST /auth/login: 5 req/min per IP, 3 req/min per email
//   - POST /auth/google: 10 req/min per IP
//   - GET  /whitelist:  30 req/min per IP
func NewAuthHandler(uc auth.AuthUsecase, env string, logger zerolog.Logger, tr tracer.Tracer) *AuthHandler {
	return &AuthHandler{
		uc:           uc,
		loginIPLim:   auth.NewRateLimiter(time.Minute, 5),
		loginEmlLim:  auth.NewRateLimiter(time.Minute, 3),
		googleLim:    auth.NewRateLimiter(time.Minute, 10),
		whitelistLim: auth.NewRateLimiter(time.Minute, 30),
		cookieSecure: env != "development" && env != "local",
		logger:       logger,
		tracer:       tr,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type googleRequest struct {
	Credential string `json:"credential"`
}

type addWhitelistRequest struct {
	Email string `json:"email"`
	Notes string `json:"notes"`
}

// Login handles POST /auth/login (email/password).
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "auth.handler.Login")
	defer span.End()

	ip := clientIP(r)
	if ok := h.applyRateLimit(w, h.loginIPLim, "ip:"+ip); !ok {
		return apperror.TooManyRequests("rate_limited")
	}

	var body loginRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	if !looksLikeEmail(body.Email) || strings.TrimSpace(body.Password) == "" {
		return apperror.ValidationError("email and password are required")
	}

	if ok := h.applyRateLimit(w, h.loginEmlLim, "email:"+strings.ToLower(body.Email)); !ok {
		return apperror.TooManyRequests("rate_limited")
	}

	res, err := h.uc.LoginEmailPassword(ctx, body.Email, body.Password)
	if err != nil {
		return mapAuthError(err)
	}

	h.setSessionCookie(w, res.SessionToken, res.ExpiresAt)

	return response.StandardSuccess(w, r, http.StatusOK, "Login berhasil", map[string]interface{}{
		"access_token": res.SessionToken,
		"expires":      res.ExpiresAt,
		"provider":     res.Provider,
		"admin":        res.IsAdmin,
		"user":         res.User,
	})
}

// LoginGoogle handles POST /auth/google.
func (h *AuthHandler) LoginGoogle(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "auth.handler.LoginGoogle")
	defer span.End()

	ip := clientIP(r)
	if ok := h.applyRateLimit(w, h.googleLim, "ip:"+ip); !ok {
		return apperror.TooManyRequests("rate_limited")
	}

	var body googleRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	if strings.TrimSpace(body.Credential) == "" {
		return apperror.ValidationError("Google credential tidak ditemukan")
	}

	res, err := h.uc.LoginGoogle(ctx, body.Credential)
	if err != nil {
		return mapAuthError(err)
	}

	h.setSessionCookie(w, res.SessionToken, res.ExpiresAt)

	return response.StandardSuccess(w, r, http.StatusOK, "Google login berhasil", map[string]interface{}{
		"user":     res.User,
		"admin":    res.IsAdmin,
		"provider": res.Provider,
	})
}

// Logout handles POST /auth/logout. It clears the session cookie and is idempotent.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return response.StandardSuccess(w, r, http.StatusOK, "Logged out", nil)
}

// ListWhitelist handles GET /whitelist (admin only — JWT middleware applied at the route level).
// Returns full email entries; must NOT be exposed publicly to prevent account enumeration.
func (h *AuthHandler) ListWhitelist(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "auth.handler.ListWhitelist")
	defer span.End()

	if ok := h.applyRateLimit(w, h.whitelistLim, "ip:"+clientIP(r)); !ok {
		return apperror.TooManyRequests("rate_limited")
	}

	entries, err := h.uc.ListWhitelist(ctx)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Whitelist retrieved", entries)
}

// CheckWhitelist handles GET /whitelist/check?email=... — public probe that returns
// only {allowed: bool}. Constant-shape response to avoid enumeration via timing/error.
func (h *AuthHandler) CheckWhitelist(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "auth.handler.CheckWhitelist")
	defer span.End()

	if ok := h.applyRateLimit(w, h.whitelistLim, "ip:"+clientIP(r)); !ok {
		return apperror.TooManyRequests("rate_limited")
	}

	email := strings.TrimSpace(r.URL.Query().Get("email"))
	allowed := false
	if looksLikeEmail(email) {
		ok, err := h.uc.IsWhitelisted(ctx, email)
		if err != nil {
			return err
		}
		allowed = ok
	}
	return response.StandardSuccess(w, r, http.StatusOK, "ok", map[string]bool{"allowed": allowed})
}

// AddWhitelist handles POST /whitelist (admin only — JWT middleware applied at the route level).
func (h *AuthHandler) AddWhitelist(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "auth.handler.AddWhitelist")
	defer span.End()

	var body addWhitelistRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	if !looksLikeEmail(body.Email) {
		return apperror.ValidationError("valid email is required")
	}

	addedBy := actorEmailFromCtx(r)
	entry, err := h.uc.AddWhitelist(ctx, body.Email, addedBy, body.Notes)
	if err != nil {
		if errors.Is(err, repository.ErrWhitelistDuplicate) {
			return apperror.Conflict("Email sudah ada di whitelist")
		}
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Whitelist created", entry)
}

// DeleteWhitelist handles DELETE /whitelist/{id} (admin only).
func (h *AuthHandler) DeleteWhitelist(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "auth.handler.DeleteWhitelist")
	defer span.End()

	id := router.GetParam(r, "id")
	if strings.TrimSpace(id) == "" {
		return apperror.ValidationError("id path param is required")
	}
	if err := h.uc.RemoveWhitelist(ctx, id); err != nil {
		if errors.Is(err, repository.ErrWhitelistNotFound) {
			return apperror.NotFound("whitelist", "")
		}
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Email dihapus dari whitelist", map[string]string{"id": id})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	cookie := &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
	if !expiresAt.IsZero() {
		cookie.Expires = expiresAt
		cookie.MaxAge = int(time.Until(expiresAt).Seconds())
	} else {
		cookie.MaxAge = int((30 * 24 * time.Hour).Seconds())
	}
	http.SetCookie(w, cookie)
}

func (h *AuthHandler) applyRateLimit(w http.ResponseWriter, lim *auth.RateLimiter, key string) bool {
	allowed, remaining, reset := lim.Allow(key)
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(lim.Limit()))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
	return allowed
}

func clientIP(r *http.Request) string {
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		// Take the first hop.
		if idx := strings.Index(v, ","); idx >= 0 {
			return strings.TrimSpace(v[:idx])
		}
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return strings.TrimSpace(v)
	}
	if r.RemoteAddr != "" {
		// Strip the port if present.
		if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > 0 {
			return r.RemoteAddr[:idx]
		}
		return r.RemoteAddr
	}
	return "unknown"
}

func looksLikeEmail(s string) bool {
	s = strings.TrimSpace(s)
	at := strings.Index(s, "@")
	if at <= 0 || at == len(s)-1 {
		return false
	}
	if strings.Contains(s, " ") {
		return false
	}
	if !strings.Contains(s[at+1:], ".") {
		return false
	}
	return true
}

// actorEmailFromCtx pulls the email of the JWT-authenticated caller from the
// JWT middleware context. Never trusts client-supplied headers.
func actorEmailFromCtx(r *http.Request) string {
	if u, ok := middleware.GetJWTUser(r.Context()); ok && u != nil {
		return u.Email
	}
	return ""
}

// mapAuthError converts usecase / repo errors into HTTP-friendly apperrors.
func mapAuthError(err error) error {
	switch {
	case errors.Is(err, auth.ErrNotWhitelisted):
		return apperror.Forbidden("not_whitelisted")
	case errors.Is(err, auth.ErrInvalidCredentials):
		return apperror.Unauthorized("Email atau password salah")
	case errors.Is(err, auth.ErrGoogleInvalidToken):
		return apperror.Unauthorized("Token Google tidak valid atau sudah expired")
	case errors.Is(err, auth.ErrProxyUnavailable):
		return apperror.InternalErrorWithMessage("Auth service unavailable", err)
	default:
		return apperror.InternalError(fmt.Errorf("auth login: %w", err))
	}
}
