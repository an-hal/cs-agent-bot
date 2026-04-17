package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/notification"
	"github.com/rs/zerolog"
)

type NotificationHandler struct {
	uc     notification.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewNotificationHandler(uc notification.Usecase, logger zerolog.Logger, tr tracer.Tracer) *NotificationHandler {
	return &NotificationHandler{uc: uc, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List notifications for the caller in this workspace
// @Tags         Notification
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true   "Workspace ID"
// @Param        unread_only     query   bool    false  "Filter to unread only"
// @Param        type            query   string  false  "Filter by notification type"
// @Param        limit           query   int     false  "Max items (default 50, max 200)"
// @Param        offset          query   int     false  "Offset"
// @Success      200             {object}  response.StandardResponseWithMeta{data=[]entity.Notification}
// @Router       /api/notifications [get]
func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.NotificationList")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := entity.NotificationFilter{
		WorkspaceID:    wsID,
		RecipientEmail: callerEmail(r),
		UnreadOnly:     r.URL.Query().Get("unread_only") == "true",
		Type:           r.URL.Query().Get("type"),
		Limit:          limit,
		Offset:         offset,
	}
	out, total, err := h.uc.List(ctx, filter)
	if err != nil {
		return err
	}
	if out == nil {
		out = []entity.Notification{}
	}
	meta := pagination.Meta{Total: total, Offset: offset, Limit: limit}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Notifications", meta, out)
}

// Count godoc
// @Summary      Count unread notifications
// @Tags         Notification
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/notifications/count [get]
func (h *NotificationHandler) Count(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.NotificationCount")
	defer span.End()

	n, err := h.uc.CountUnread(ctx, ctxutil.GetWorkspaceID(ctx), callerEmail(r))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Unread count", map[string]int64{"unread": n})
}

// MarkRead godoc
// @Summary      Mark a notification as read
// @Tags         Notification
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Param        id              path    string  true  "Notification ID"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/notifications/{id}/read [put]
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.NotificationMarkRead")
	defer span.End()

	id := router.GetParam(r, "id")
	if err := h.uc.MarkRead(ctx, id, callerEmail(r)); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Notification marked read", nil)
}

// MarkAllRead godoc
// @Summary      Mark all notifications as read
// @Tags         Notification
// @Security     BearerAuth
// @Param        X-Workspace-ID  header  string  true  "Workspace ID"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/notifications/read-all [put]
func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.NotificationMarkAllRead")
	defer span.End()

	if err := h.uc.MarkAllRead(ctx, ctxutil.GetWorkspaceID(ctx), callerEmail(r)); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "All notifications marked read", nil)
}

// Create godoc
// @Summary      Publish a notification (internal)
// @Tags         Notification
// @Security     BearerAuth
// @Accept       json
// @Param        X-Workspace-ID  header  string                       true  "Workspace ID"
// @Param        body            body    notification.CreateRequest   true  "Notification payload"
// @Success      201  {object}  response.StandardResponse{data=entity.Notification}
// @Router       /api/notifications [post]
func (h *NotificationHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.NotificationCreate")
	defer span.End()

	var req notification.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	}
	out, err := h.uc.Create(ctx, req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Notification created", out)
}
