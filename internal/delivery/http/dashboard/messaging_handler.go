package dashboard

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/messaging"
	"github.com/rs/zerolog"
)

// MessagingHandler exposes the /templates/* endpoints defined by the
// 05-messaging spec.
type MessagingHandler struct {
	uc     messaging.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewMessagingHandler(uc messaging.Usecase, logger zerolog.Logger, tr tracer.Tracer) *MessagingHandler {
	return &MessagingHandler{uc: uc, logger: logger, tracer: tr}
}

type listMeta struct {
	Total int `json:"total"`
}

// ------------------- Message templates -------------------

// ListMessages godoc
// @Summary      List message templates
// @Description  List workspace-scoped WA/Telegram templates.
// @Tags         Messaging
// @Param        role      query  string  false  "sdr | bd | ae"
// @Param        phase     query  string  false  "Comma-separated phases (P0,P1)"
// @Param        channel   query  string  false  "whatsapp | telegram"
// @Param        category  query  string  false  "Category filter"
// @Param        search    query  string  false  "Search id/action/message"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.MessageTemplate}
// @Router       /api/templates/messages [get]
func (h *MessagingHandler) ListMessages(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.ListMessages")
	defer span.End()
	q := r.URL.Query()
	filter := entity.MessageTemplateFilter{
		Role:     q.Get("role"),
		Channel:  q.Get("channel"),
		Category: q.Get("category"),
		Search:   q.Get("search"),
	}
	if p := q.Get("phase"); p != "" {
		filter.Phases = strings.Split(p, ",")
	}
	out, err := h.uc.ListMessageTemplates(ctx, ctxutil.GetWorkspaceID(ctx), filter)
	if err != nil {
		return err
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Message templates", listMeta{Total: len(out)}, out)
}

// GetMessage godoc
// @Summary      Get message template by ID
// @Tags         Messaging
// @Param        id   path  string  true  "Template ID"
// @Success      200  {object}  response.StandardResponse{data=entity.MessageTemplate}
// @Router       /api/templates/messages/{id} [get]
func (h *MessagingHandler) GetMessage(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.GetMessage")
	defer span.End()
	id := router.GetParam(r, "id")
	t, err := h.uc.GetMessageTemplate(ctx, ctxutil.GetWorkspaceID(ctx), id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Message template", t)
}

// CreateMessage godoc
// @Summary      Create message template
// @Tags         Messaging
// @Param        body  body  entity.MessageTemplate  true  "Template body"
// @Success      201  {object}  response.StandardResponse{data=entity.MessageTemplate}
// @Router       /api/templates/messages [post]
func (h *MessagingHandler) CreateMessage(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.CreateMessage")
	defer span.End()
	var body entity.MessageTemplate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	body.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	out, err := h.uc.CreateMessageTemplate(ctx, editorFromCtx(r), &body)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Message template created", out)
}

// UpdateMessage godoc
// @Summary      Update message template
// @Tags         Messaging
// @Param        id    path  string                  true  "Template ID"
// @Param        body  body  entity.MessageTemplate  true  "Partial update"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/templates/messages/{id} [put]
func (h *MessagingHandler) UpdateMessage(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.UpdateMessage")
	defer span.End()
	id := router.GetParam(r, "id")
	var body entity.MessageTemplate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	body.ID = id
	body.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	updated, changed, err := h.uc.UpdateMessageTemplate(ctx, editorFromCtx(r), &body)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Message template updated", map[string]interface{}{
		"data":           updated,
		"changed_fields": changed,
	})
}

// DeleteMessage godoc
// @Summary      Delete message template
// @Tags         Messaging
// @Param        id   path  string  true  "Template ID"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/templates/messages/{id} [delete]
func (h *MessagingHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.DeleteMessage")
	defer span.End()
	id := router.GetParam(r, "id")
	if err := h.uc.DeleteMessageTemplate(ctx, editorFromCtx(r), ctxutil.GetWorkspaceID(ctx), id); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Deleted", map[string]string{"id": id})
}

// ------------------- Email templates -------------------

// ListEmails godoc
// @Summary      List email templates
// @Tags         Messaging
// @Param        role      query  string  false  "sdr | bd | ae"
// @Param        category  query  string  false  "Category filter"
// @Param        status    query  string  false  "active | draft | archived"
// @Param        search    query  string  false  "Search id/name/subject"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.EmailTemplate}
// @Router       /api/templates/emails [get]
func (h *MessagingHandler) ListEmails(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.ListEmails")
	defer span.End()
	q := r.URL.Query()
	filter := entity.EmailTemplateFilter{
		Role:     q.Get("role"),
		Category: q.Get("category"),
		Status:   q.Get("status"),
		Search:   q.Get("search"),
	}
	out, err := h.uc.ListEmailTemplates(ctx, ctxutil.GetWorkspaceID(ctx), filter)
	if err != nil {
		return err
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Email templates", listMeta{Total: len(out)}, out)
}

// GetEmail godoc
// @Summary      Get email template by ID
// @Tags         Messaging
// @Param        id   path  string  true  "Template ID"
// @Success      200  {object}  response.StandardResponse{data=entity.EmailTemplate}
// @Router       /api/templates/emails/{id} [get]
func (h *MessagingHandler) GetEmail(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.GetEmail")
	defer span.End()
	id := router.GetParam(r, "id")
	t, err := h.uc.GetEmailTemplate(ctx, ctxutil.GetWorkspaceID(ctx), id)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Email template", t)
}

// CreateEmail godoc
// @Summary      Create email template
// @Tags         Messaging
// @Param        body  body  entity.EmailTemplate  true  "Template body"
// @Success      201  {object}  response.StandardResponse{data=entity.EmailTemplate}
// @Router       /api/templates/emails [post]
func (h *MessagingHandler) CreateEmail(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.CreateEmail")
	defer span.End()
	var body entity.EmailTemplate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	body.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	out, err := h.uc.CreateEmailTemplate(ctx, editorFromCtx(r), &body)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Email template created", out)
}

// UpdateEmail godoc
// @Summary      Update email template
// @Tags         Messaging
// @Param        id    path  string                true  "Template ID"
// @Param        body  body  entity.EmailTemplate  true  "Partial update"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/templates/emails/{id} [put]
func (h *MessagingHandler) UpdateEmail(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.UpdateEmail")
	defer span.End()
	id := router.GetParam(r, "id")
	var body entity.EmailTemplate
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	body.ID = id
	body.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	updated, changed, err := h.uc.UpdateEmailTemplate(ctx, editorFromCtx(r), &body)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Email template updated", map[string]interface{}{
		"data":           updated,
		"changed_fields": changed,
	})
}

// DeleteEmail godoc
// @Summary      Delete email template
// @Tags         Messaging
// @Param        id   path  string  true  "Template ID"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/templates/emails/{id} [delete]
func (h *MessagingHandler) DeleteEmail(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.DeleteEmail")
	defer span.End()
	id := router.GetParam(r, "id")
	if err := h.uc.DeleteEmailTemplate(ctx, editorFromCtx(r), ctxutil.GetWorkspaceID(ctx), id); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Deleted", map[string]string{"id": id})
}

// ------------------- Preview + edit logs + variables -------------------

// Preview godoc
// @Summary      Render template with sample data
// @Tags         Messaging
// @Param        body  body  messaging.PreviewRequest  true  "Preview request"
// @Success      200  {object}  response.StandardResponse{data=messaging.PreviewResult}
// @Router       /api/templates/preview [post]
func (h *MessagingHandler) Preview(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.Preview")
	defer span.End()
	var req messaging.PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.ValidationError("Invalid request body")
	}
	out, err := h.uc.Preview(ctx, ctxutil.GetWorkspaceID(ctx), req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Preview", out)
}

// ListEditLogs godoc
// @Summary      List template edit logs
// @Tags         Messaging
// @Param        template_id    query  string  false  "Filter by template_id"
// @Param        template_type  query  string  false  "message | email"
// @Param        limit          query  int     false  "Max rows (default 50)"
// @Param        since          query  string  false  "RFC3339 timestamp"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.TemplateEditLog}
// @Router       /api/templates/edit-logs [get]
func (h *MessagingHandler) ListEditLogs(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.ListEditLogs")
	defer span.End()
	q := r.URL.Query()
	filter := entity.TemplateEditLogFilter{
		TemplateID:   q.Get("template_id"),
		TemplateType: q.Get("template_type"),
	}
	if l := q.Get("limit"); l != "" {
		if n, err := parsePositiveInt(l); err == nil {
			filter.Limit = n
		}
	}
	if s := q.Get("since"); s != "" {
		if ts, err := time.Parse(time.RFC3339, s); err == nil {
			filter.Since = &ts
		}
	}
	out, err := h.uc.ListEditLogs(ctx, ctxutil.GetWorkspaceID(ctx), filter)
	if err != nil {
		return err
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Edit logs", listMeta{Total: len(out)}, out)
}

// GetEditLogsForTemplate godoc
// @Summary      List edit logs for a specific template
// @Tags         Messaging
// @Param        template_id  path  string  true  "Template ID"
// @Param        limit        query int     false "Max rows"
// @Success      200  {object}  response.StandardResponse{data=[]entity.TemplateEditLog}
// @Router       /api/templates/edit-logs/{template_id} [get]
func (h *MessagingHandler) GetEditLogsForTemplate(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.GetEditLogsForTemplate")
	defer span.End()
	filter := entity.TemplateEditLogFilter{TemplateID: router.GetParam(r, "template_id")}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := parsePositiveInt(l); err == nil {
			filter.Limit = n
		}
	}
	out, err := h.uc.ListEditLogs(ctx, ctxutil.GetWorkspaceID(ctx), filter)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Edit logs", out)
}

// ListVariables godoc
// @Summary      List template variables available in this workspace
// @Tags         Messaging
// @Success      200  {object}  response.StandardResponse{data=[]entity.TemplateVariable}
// @Router       /api/templates/variables [get]
func (h *MessagingHandler) ListVariables(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "messaging.handler.ListVariables")
	defer span.End()
	out, err := h.uc.ListVariables(ctx, ctxutil.GetWorkspaceID(ctx))
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Template variables", out)
}

// editorFromCtx returns the editor's identifier for edit-log attribution.
func editorFromCtx(r *http.Request) string {
	if a := actorFromCtx(r); a != "" {
		return a
	}
	return "system"
}

func parsePositiveInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, apperror.ValidationError("expected integer")
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return 0, apperror.ValidationError("expected positive integer")
	}
	return n, nil
}
