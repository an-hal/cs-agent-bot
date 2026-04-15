package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/middleware"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	automationrule "github.com/Sejutacita/cs-agent-bot/internal/usecase/automation_rule"
	"github.com/rs/zerolog"
)

// AutomationRuleHandler handles CRUD for automation rules and change logs.
type AutomationRuleHandler struct {
	uc     automationrule.Usecase
	logger zerolog.Logger
	tracer tracer.Tracer
}

func NewAutomationRuleHandler(uc automationrule.Usecase, logger zerolog.Logger, tr tracer.Tracer) *AutomationRuleHandler {
	return &AutomationRuleHandler{uc: uc, logger: logger, tracer: tr}
}

// arActorFromCtx extracts the actor email for automation rule operations.
func arActorFromCtx(r *http.Request) string {
	if u, ok := middleware.GetJWTUser(r.Context()); ok {
		return u.Email
	}
	return "unknown"
}

// ─── List ─────────────────────────────────────────────────────────────────────

// ListRules godoc
// @Summary      List automation rules
// @Tags         AutomationRules
// @Param        role    query  string  false  "sdr | bd | ae | cs"
// @Param        status  query  string  false  "active | paused | disabled"
// @Param        phase   query  string  false  "Phase filter (P0, P1, ...)"
// @Param        search  query  string  false  "Search trigger_id, template_id, condition, rule_code"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.AutomationRule}
// @Router       /api/automation-rules [get]
func (h *AutomationRuleHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "automationRule.handler.List")
	defer span.End()

	q := r.URL.Query()
	filter := entity.AutomationRuleFilter{
		Role:   q.Get("role"),
		Status: q.Get("status"),
		Phase:  q.Get("phase"),
		Search: q.Get("search"),
	}
	wsID := ctxutil.GetWorkspaceID(ctx)

	rules, err := h.uc.List(ctx, wsID, filter)
	if err != nil {
		return err
	}

	type meta struct {
		Total int `json:"total"`
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Automation rules", meta{Total: len(rules)}, rules)
}

// ─── GetByID ──────────────────────────────────────────────────────────────────

// GetRule godoc
// @Summary      Get automation rule by ID
// @Description  Returns rule with change logs.
// @Tags         AutomationRules
// @Param        id  path  string  true  "Rule ID"
// @Success      200  {object}  response.StandardResponse
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/automation-rules/{id} [get]
func (h *AutomationRuleHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "automationRule.handler.Get")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	rule, logs, err := h.uc.GetByID(ctx, wsID, id)
	if err != nil {
		return err
	}

	type ruleWithLogs struct {
		Data       *entity.AutomationRule   `json:"data"`
		ChangeLogs []entity.RuleChangeLog   `json:"change_logs"`
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Automation rule", ruleWithLogs{
		Data:       rule,
		ChangeLogs: logs,
	})
}

// ─── Create ───────────────────────────────────────────────────────────────────

// CreateRule godoc
// @Summary      Create automation rule
// @Tags         AutomationRules
// @Param        body  body  entity.AutomationRule  true  "Rule data"
// @Success      201   {object}  response.StandardResponse{data=entity.AutomationRule}
// @Failure      400   {object}  response.StandardResponse
// @Failure      409   {object}  response.StandardResponse "Duplicate rule_code"
// @Router       /api/automation-rules [post]
func (h *AutomationRuleHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "automationRule.handler.Create")
	defer span.End()

	var req entity.AutomationRule
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return err
	}
	req.WorkspaceID = ctxutil.GetWorkspaceID(ctx)
	actor := arActorFromCtx(r)

	created, err := h.uc.Create(ctx, actor, &req)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusCreated, "Automation rule created", created)
}

// ─── Update ───────────────────────────────────────────────────────────────────

// UpdateRule godoc
// @Summary      Update automation rule
// @Description  Updates rule fields and automatically appends change log entries.
// @Tags         AutomationRules
// @Param        id    path  string          true  "Rule ID"
// @Param        body  body  map[string]any  true  "Fields to update"
// @Success      200   {object}  response.StandardResponse
// @Failure      404   {object}  response.StandardResponse
// @Router       /api/automation-rules/{id} [put]
func (h *AutomationRuleHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "automationRule.handler.Update")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)
	actor := arActorFromCtx(r)

	var fields map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		return err
	}

	updated, changes, err := h.uc.Update(ctx, wsID, id, actor, fields)
	if err != nil {
		return err
	}

	type updateResp struct {
		Data    *entity.AutomationRule   `json:"data"`
		Changes []entity.RuleChangeLog   `json:"changes"`
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Updated", updateResp{Data: updated, Changes: changes})
}

// ─── Delete ───────────────────────────────────────────────────────────────────

// DeleteRule godoc
// @Summary      Delete automation rule
// @Tags         AutomationRules
// @Param        id  path  string  true  "Rule ID"
// @Success      200  {object}  response.StandardResponse
// @Failure      404  {object}  response.StandardResponse
// @Router       /api/automation-rules/{id} [delete]
func (h *AutomationRuleHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "automationRule.handler.Delete")
	defer span.End()

	id := router.GetParam(r, "id")
	wsID := ctxutil.GetWorkspaceID(ctx)

	if err := h.uc.Delete(ctx, wsID, id); err != nil {
		return err
	}
	type deleted struct {
		Message string `json:"message"`
		ID      string `json:"id"`
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Deleted", deleted{Message: "Deleted", ID: id})
}

// ─── Change logs ──────────────────────────────────────────────────────────────

// ListChangeLogs godoc
// @Summary      Get automation rule change log feed
// @Tags         AutomationRules
// @Param        limit  query  int  false  "Max number of entries (default 50)"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.RuleChangeLogWithCode}
// @Router       /api/automation-rules/change-logs [get]
func (h *AutomationRuleHandler) ListChangeLogs(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "automationRule.handler.ListChangeLogs")
	defer span.End()

	wsID := ctxutil.GetWorkspaceID(ctx)
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	logs, err := h.uc.ListChangeLogs(ctx, wsID, limit)
	if err != nil {
		return err
	}
	if logs == nil {
		logs = []entity.RuleChangeLogWithCode{}
	}

	type meta struct {
		Total int `json:"total"`
	}
	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Change logs", meta{Total: len(logs)}, logs)
}
