package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/ctxutil"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/template"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/trigger"
	"github.com/rs/zerolog"
)

// TriggerRuleHandler handles CRUD for trigger rules and template variables.
type TriggerRuleHandler struct {
	repo       repository.TriggerRuleRepository
	logRepo    repository.LogRepository
	ruleEngine *trigger.RuleEngine
	logger     zerolog.Logger
	tracer     tracer.Tracer
}

func NewTriggerRuleHandler(
	repo repository.TriggerRuleRepository,
	logRepo repository.LogRepository,
	ruleEngine *trigger.RuleEngine,
	logger zerolog.Logger,
	tr tracer.Tracer,
) *TriggerRuleHandler {
	return &TriggerRuleHandler{repo: repo, logRepo: logRepo, ruleEngine: ruleEngine, logger: logger, tracer: tr}
}

// List godoc
// @Summary      List trigger rules
// @Tags         Dashboard
// @Param        rule_group   query  string  false  "Filter by rule group"
// @Param        action_type  query  string  false  "Filter by action type"
// @Param        active       query  string  false  "Filter by active status"
// @Param        search       query  string  false  "Search by rule_id, group, description"
// @Param        offset       query  int     false  "Pagination offset"
// @Param        limit        query  int     false  "Limit per page"
// @Success      200  {object}  response.StandardResponseWithMeta{data=[]entity.TriggerRule}
// @Router       /api/data-master/trigger-rules [get]
func (h *TriggerRuleHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.TriggerRuleList")
	defer span.End()

	params := pagination.FromRequest(r)
	q := r.URL.Query()

	filter := entity.TriggerRuleFilter{
		RuleGroup:  q.Get("rule_group"),
		ActionType: q.Get("action_type"),
		Search:     q.Get("search"),
	}
	if activeStr := q.Get("active"); activeStr != "" {
		v, err := strconv.ParseBool(activeStr)
		if err == nil {
			filter.Active = &v
		}
	}

	rules, total, err := h.repo.GetAllPaginated(ctx, filter, params)
	if err != nil {
		return err
	}
	if rules == nil {
		rules = []entity.TriggerRule{}
	}

	return response.StandardSuccessWithMeta(w, r, http.StatusOK, "Trigger rules", pagination.NewMeta(params, total), rules)
}

// Get godoc
// @Summary      Get trigger rule by ID
// @Tags         Dashboard
// @Param        rule_id  path  string  true  "Rule ID"
// @Success      200  {object}  response.StandardResponse{data=entity.TriggerRule}
// @Router       /api/data-master/trigger-rules/{rule_id} [get]
func (h *TriggerRuleHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.TriggerRuleGet")
	defer span.End()

	ruleID := router.GetParam(r, "rule_id")
	rule, err := h.repo.GetByID(ctx, ruleID)
	if err != nil {
		return err
	}
	if rule == nil {
		return apperror.NotFound("trigger_rule", "Trigger rule not found")
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Trigger rule", rule)
}

// Create godoc
// @Summary      Create trigger rule
// @Tags         Dashboard
// @Param        body  body  entity.TriggerRule  true  "Trigger rule"
// @Success      201  {object}  response.StandardResponse
// @Router       /api/data-master/trigger-rules [post]
func (h *TriggerRuleHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.TriggerRuleCreate")
	defer span.End()

	var rule entity.TriggerRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	if rule.RuleID == "" || rule.RuleGroup == "" || rule.FlagKey == "" || rule.ActionType == "" {
		return apperror.ValidationError("rule_id, rule_group, flag_key, and action_type are required")
	}

	// Validate condition JSON is valid
	if rule.Condition == nil {
		return apperror.ValidationError("condition is required")
	}
	var condCheck map[string]interface{}
	if err := json.Unmarshal(rule.Condition, &condCheck); err != nil {
		return apperror.ValidationError("condition must be valid JSON: " + err.Error())
	}

	rule.Active = true
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	if err := h.repo.Create(ctx, rule); err != nil {
		return err
	}

	// Invalidate rule cache
	if h.ruleEngine != nil {
		h.ruleEngine.InvalidateCache()
	}

	if err := h.logRepo.AppendActivity(ctx, entity.ActivityLog{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		Category:     entity.ActivityCategoryData,
		ActorType:    entity.ActivityActorHuman,
		Actor:        actorFromCtx(r),
		Action:       "add_trigger_rule",
		Target:       rule.RuleID,
		RefID:        rule.RuleID,
		ResourceType: entity.ActivityResourceTriggerRule,
		Detail:       fmt.Sprintf("group=%s action_type=%s", rule.RuleGroup, rule.ActionType),
	}); err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusCreated, "Trigger rule created", rule)
}

// Update godoc
// @Summary      Update trigger rule
// @Tags         Dashboard
// @Param        rule_id  path  string  true  "Rule ID"
// @Param        body     body  map[string]interface{}  true  "Fields to update"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/data-master/trigger-rules/{rule_id} [put]
func (h *TriggerRuleHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.TriggerRuleUpdate")
	defer span.End()

	ruleID := router.GetParam(r, "rule_id")

	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		return apperror.ValidationError("Invalid request body")
	}

	// Only allow safe fields
	allowed := map[string]bool{
		"rule_group":    true,
		"priority":      true,
		"sub_priority":  true,
		"condition":     true,
		"action_type":   true,
		"template_id":   true,
		"flag_key":      true,
		"escalation_id": true,
		"esc_priority":  true,
		"esc_reason":    true,
		"extra_flags":   true,
		"stop_on_fire":  true,
		"active":        true,
		"description":   true,
	}
	for key := range patch {
		if !allowed[key] {
			delete(patch, key)
		}
	}

	// Validate condition if provided
	if condRaw, ok := patch["condition"]; ok {
		condBytes, err := json.Marshal(condRaw)
		if err != nil {
			return apperror.ValidationError("condition must be valid JSON")
		}
		var condCheck map[string]interface{}
		if err := json.Unmarshal(condBytes, &condCheck); err != nil {
			return apperror.ValidationError("condition must be valid JSON: " + err.Error())
		}
	}

	if err := h.repo.Update(ctx, ruleID, patch); err != nil {
		return err
	}

	// Invalidate rule cache
	if h.ruleEngine != nil {
		h.ruleEngine.InvalidateCache()
	}

	keys := make([]string, 0, len(patch))
	for k := range patch {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if err := h.logRepo.AppendActivity(ctx, entity.ActivityLog{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		Category:     entity.ActivityCategoryData,
		ActorType:    entity.ActivityActorHuman,
		Actor:        actorFromCtx(r),
		Action:       "edit_trigger_rule",
		Target:       ruleID,
		RefID:        ruleID,
		ResourceType: entity.ActivityResourceTriggerRule,
		Detail:       "Ubah: " + strings.Join(keys, ", "),
	}); err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Trigger rule updated", nil)
}

// Delete godoc
// @Summary      Delete trigger rule (soft delete)
// @Tags         Dashboard
// @Param        rule_id  path  string  true  "Rule ID"
// @Success      200  {object}  response.StandardResponse
// @Router       /api/data-master/trigger-rules/{rule_id} [delete]
func (h *TriggerRuleHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, span := h.tracer.Start(r.Context(), "dashboard.handler.TriggerRuleDelete")
	defer span.End()

	ruleID := router.GetParam(r, "rule_id")
	if err := h.repo.Delete(ctx, ruleID); err != nil {
		return err
	}

	if h.ruleEngine != nil {
		h.ruleEngine.InvalidateCache()
	}

	if err := h.logRepo.AppendActivity(ctx, entity.ActivityLog{
		WorkspaceID:  ctxutil.GetWorkspaceID(ctx),
		Category:     entity.ActivityCategoryData,
		ActorType:    entity.ActivityActorHuman,
		Actor:        actorFromCtx(r),
		Action:       "delete_trigger_rule",
		Target:       ruleID,
		RefID:        ruleID,
		ResourceType: entity.ActivityResourceTriggerRule,
	}); err != nil {
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Trigger rule deleted", nil)
}

// InvalidateCache godoc
// @Summary      Invalidate trigger rule cache
// @Tags         Dashboard
// @Success      200  {object}  response.StandardResponse
// @Router       /api/data-master/trigger-rules/cache/invalidate [post]
func (h *TriggerRuleHandler) InvalidateCache(w http.ResponseWriter, r *http.Request) error {
	_, span := h.tracer.Start(r.Context(), "dashboard.handler.TriggerRuleInvalidateCache")
	defer span.End()

	if h.ruleEngine != nil {
		h.ruleEngine.InvalidateCache()
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Cache invalidated", nil)
}

// ListVariables godoc
// @Summary      List available template variables
// @Tags         Dashboard
// @Param        channel  query  string  false  "Filter by channel (wa, email)"
// @Success      200  {object}  response.StandardResponse{data=[]template.VariableInfo}
// @Router       /api/data-master/template-variables [get]
func (h *TriggerRuleHandler) ListVariables(w http.ResponseWriter, r *http.Request) error {
	_, span := h.tracer.Start(r.Context(), "dashboard.handler.ListTemplateVariables")
	defer span.End()

	channel := r.URL.Query().Get("channel")

	var variables []template.VariableInfo
	if channel != "" {
		variables = template.GetAvailableVariablesByChannel(channel)
	} else {
		variables = template.GetAvailableVariables()
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Template variables", variables)
}
