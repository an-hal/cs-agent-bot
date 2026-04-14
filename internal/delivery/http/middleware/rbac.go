package middleware

import (
	"context"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/team"
)

// rbacContextKey is an unexported type to avoid collisions on the request
// context when stashing resolved permission scope.
type rbacContextKey string

const scopeContextKey rbacContextKey = "rbac_scope"

// WithScope stores the resolved view-list scope (false/true/all) on the
// request context so downstream handlers can broaden queries for holding-level
// visibility.
func WithScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, scopeContextKey, scope)
}

// GetScope reads the resolved view-list scope from the request context.
// Returns "" when the middleware did not run.
func GetScope(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(scopeContextKey).(string); ok {
		return v
	}
	return ""
}

// RequirePermission returns a middleware that enforces `module.action` against
// the caller's role in the current workspace. On success, the resolved
// view-list scope is stored in the context so handlers can decide whether to
// filter by workspace or return holding-wide data.
//
// The middleware must run after both JWTAuthMiddleware and
// WorkspaceIDMiddleware have populated the request context.
func RequirePermission(module, action string, uc team.Usecase) func(ErrorHandler) ErrorHandler {
	return func(next ErrorHandler) ErrorHandler {
		return func(w http.ResponseWriter, r *http.Request) error {
			user, ok := GetJWTUser(r.Context())
			if !ok || user.Email == "" {
				return apperror.Unauthorized("missing caller")
			}
			workspaceID := ctxutil.GetWorkspaceID(r.Context())
			if workspaceID == "" {
				return apperror.BadRequest("X-Workspace-ID header is required")
			}
			allowed, resolved, err := uc.CheckPermission(r.Context(), user.Email, workspaceID, module, action)
			if err != nil {
				return err
			}
			if !allowed {
				return apperror.Forbidden("permission denied: " + module + "." + action)
			}
			scope := entity.ViewScopeTrue
			if resolved != nil {
				scope = resolved.ViewList
			}
			ctx := WithScope(r.Context(), scope)
			return next(w, r.WithContext(ctx))
		}
	}
}
