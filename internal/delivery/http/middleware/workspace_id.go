package middleware

import (
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
)

// WorkspaceIDMiddleware extracts the X-Workspace-ID header and stores it in
// the request context. Returns 400 if the header is missing or empty.
func WorkspaceIDMiddleware() func(ErrorHandler) ErrorHandler {
	return func(next ErrorHandler) ErrorHandler {
		return func(w http.ResponseWriter, r *http.Request) error {
			workspaceID := r.Header.Get(ctxutil.WorkspaceIDHeader)
			if workspaceID == "" {
				return apperror.BadRequest("X-Workspace-ID header is required")
			}

			ctx := ctxutil.SetWorkspaceID(r.Context(), workspaceID)
			return next(w, r.WithContext(ctx))
		}
	}
}
