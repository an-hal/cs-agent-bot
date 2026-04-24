// Package automation_rule implements CRUD, change-log, and checker-maker
// toggle for automation rules (feat/06-workflow-engine).
package automation_rule

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// Usecase is the automation rule management interface.
type Usecase interface {
	List(ctx context.Context, workspaceID string, filter entity.AutomationRuleFilter) ([]entity.AutomationRule, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.AutomationRule, []entity.RuleChangeLog, error)
	Create(ctx context.Context, actor string, r *entity.AutomationRule) (*entity.AutomationRule, error)
	Update(ctx context.Context, workspaceID, id, actor string, fields map[string]interface{}) (*entity.AutomationRule, []entity.RuleChangeLog, error)
	Delete(ctx context.Context, workspaceID, id string) error

	// Change logs
	ListChangeLogs(ctx context.Context, workspaceID string, limit int) ([]entity.RuleChangeLogWithCode, error)

	// For cron
	GetActiveByRole(ctx context.Context, workspaceID string, role entity.RuleRole) ([]entity.AutomationRule, error)

	// ApplyToggleStatus executes a pending "toggle_automation_rule" approval
	// (active↔paused). The approval payload must include {rule_id, target_status}.
	ApplyToggleStatus(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.AutomationRule, error)
}

type usecase struct {
	repo        repository.AutomationRuleRepository
	approvalRepo repository.ApprovalRequestRepository
	logger      zerolog.Logger
}

// New constructs an automation rule Usecase.
func New(repo repository.AutomationRuleRepository, approvalRepo repository.ApprovalRequestRepository, logger zerolog.Logger) Usecase {
	return &usecase{repo: repo, approvalRepo: approvalRepo, logger: logger}
}

// ─── List ─────────────────────────────────────────────────────────────────────

func (u *usecase) List(ctx context.Context, workspaceID string, filter entity.AutomationRuleFilter) ([]entity.AutomationRule, error) {
	rules, err := u.repo.List(ctx, workspaceID, filter)
	if err != nil {
		return nil, fmt.Errorf("automationRule.List: %w", err)
	}
	if rules == nil {
		rules = []entity.AutomationRule{}
	}
	return rules, nil
}

// ─── GetByID ──────────────────────────────────────────────────────────────────

func (u *usecase) GetByID(ctx context.Context, workspaceID, id string) (*entity.AutomationRule, []entity.RuleChangeLog, error) {
	rule, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, nil, fmt.Errorf("automationRule.GetByID: %w", err)
	}
	if rule == nil {
		return nil, nil, apperror.NotFound("automation_rule", "Automation rule not found")
	}

	logs, err := u.repo.ListChangeLogsForRule(ctx, id)
	if err != nil {
		u.logger.Warn().Err(err).Str("rule_id", id).Msg("Failed to load change logs for rule")
		logs = []entity.RuleChangeLog{}
	}
	if logs == nil {
		logs = []entity.RuleChangeLog{}
	}
	return rule, logs, nil
}

// ─── Create ───────────────────────────────────────────────────────────────────

func (u *usecase) Create(ctx context.Context, actor string, r *entity.AutomationRule) (*entity.AutomationRule, error) {
	if err := validateRule(r); err != nil {
		return nil, err
	}
	if r.Channel == "" {
		r.Channel = entity.RuleChannelWhatsApp
	}
	if r.Status == "" {
		r.Status = entity.RuleStatusActive
	}
	if r.StopIf == "" {
		r.StopIf = "-"
	}
	r.UpdatedBy = &actor

	if err := u.repo.Create(ctx, r); err != nil {
		return nil, fmt.Errorf("automationRule.Create: %w", err)
	}
	return r, nil
}

// ─── Update ───────────────────────────────────────────────────────────────────

// Update applies patch fields. Status changes between active↔paused go through
// the approval flow (checker-maker); other field changes are applied directly.
func (u *usecase) Update(ctx context.Context, workspaceID, id, actor string, fields map[string]interface{}) (*entity.AutomationRule, []entity.RuleChangeLog, error) {
	current, _, err := u.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, nil, err
	}

	// Checker-maker gate: status toggle active↔paused requires approval
	if newStatus, ok := fields["status"]; ok {
		ns := fmt.Sprint(newStatus)
		cs := string(current.Status)
		if isToggleRequiringApproval(cs, ns) {
			if u.approvalRepo != nil {
				desc := fmt.Sprintf("Toggle automation rule %s: %s → %s", current.RuleCode, cs, ns)
				_, aerr := u.approvalRepo.Create(ctx, &entity.ApprovalRequest{
					WorkspaceID: workspaceID,
					RequestType: "toggle_automation_rule",
					Description: desc,
					MakerEmail:  actor,
					Payload: map[string]any{
						"rule_id":       current.ID,
						"rule_code":     current.RuleCode,
						"from_status":   cs,
						"target_status": ns,
					},
				})
				if aerr != nil {
					u.logger.Warn().Err(aerr).Msg("Failed to create approval request for rule toggle")
				}
			}
			return current, nil, apperror.ValidationError("status change requires approval — approval request created")
		}
	}

	logs, err := u.repo.Update(ctx, workspaceID, id, fields, actor)
	if err != nil {
		return nil, nil, fmt.Errorf("automationRule.Update: %w", err)
	}

	updated, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, nil, fmt.Errorf("automationRule.Update re-fetch: %w", err)
	}
	if logs == nil {
		logs = []entity.RuleChangeLog{}
	}
	return updated, logs, nil
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (u *usecase) Delete(ctx context.Context, workspaceID, id string) error {
	rule, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return fmt.Errorf("automationRule.Delete: %w", err)
	}
	if rule == nil {
		return apperror.NotFound("automation_rule", "Automation rule not found")
	}
	return u.repo.Delete(ctx, workspaceID, id)
}

// ─── ChangeLogs ───────────────────────────────────────────────────────────────

func (u *usecase) ListChangeLogs(ctx context.Context, workspaceID string, limit int) ([]entity.RuleChangeLogWithCode, error) {
	logs, err := u.repo.ListChangeLogs(ctx, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("automationRule.ListChangeLogs: %w", err)
	}
	if logs == nil {
		logs = []entity.RuleChangeLogWithCode{}
	}
	return logs, nil
}

// ─── GetActiveByRole (cron) ───────────────────────────────────────────────────

func (u *usecase) GetActiveByRole(ctx context.Context, workspaceID string, role entity.RuleRole) ([]entity.AutomationRule, error) {
	rules, err := u.repo.GetActiveByRole(ctx, workspaceID, role)
	if err != nil {
		return nil, fmt.Errorf("automationRule.GetActiveByRole: %w", err)
	}
	if rules == nil {
		rules = []entity.AutomationRule{}
	}
	return rules, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func validateRule(r *entity.AutomationRule) error {
	if r.RuleCode == "" {
		return apperror.ValidationError("rule_code is required")
	}
	if r.TriggerID == "" {
		return apperror.ValidationError("trigger_id is required")
	}
	if r.Timing == "" {
		return apperror.ValidationError("timing is required")
	}
	if r.Condition == "" {
		return apperror.ValidationError("condition is required")
	}
	if r.Role == "" {
		return apperror.ValidationError("role is required")
	}
	if r.Phase == "" {
		return apperror.ValidationError("phase is required")
	}
	return nil
}

// isToggleRequiringApproval checks if a status change is the active↔paused toggle
// that mandates a checker-maker approval.
func isToggleRequiringApproval(currentStatus, newStatus string) bool {
	return (currentStatus == "active" && newStatus == "paused") ||
		(currentStatus == "paused" && newStatus == "active")
}

// ApplyToggleStatus executes a pending toggle_automation_rule approval.
// It flips the rule's status to payload.target_status, records the change log
// via repo.Update, then marks the approval approved. Same constraints as
// collection.ApplyCollectionSchemaChange: maker != checker, status=pending,
// request_type matches.
func (u *usecase) ApplyToggleStatus(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.AutomationRule, error) {
	if workspaceID == "" || approvalID == "" {
		return nil, apperror.ValidationError("workspace_id and approval_id required")
	}
	if u.approvalRepo == nil {
		return nil, apperror.InternalErrorWithMessage("approval repo not wired", nil)
	}
	ar, err := u.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return nil, err
	}
	if ar == nil {
		return nil, apperror.NotFound("approval_request", approvalID)
	}
	if ar.RequestType != "toggle_automation_rule" {
		return nil, apperror.BadRequest("approval is not a toggle_automation_rule request")
	}
	if ar.Status != entity.ApprovalStatusPending {
		return nil, apperror.BadRequest("approval is not pending (status=" + ar.Status + ")")
	}
	if ar.MakerEmail == checkerEmail {
		return nil, apperror.BadRequest("cannot approve your own request")
	}

	ruleID, _ := ar.Payload["rule_id"].(string)
	targetStatus, _ := ar.Payload["target_status"].(string)
	if ruleID == "" || targetStatus == "" {
		return nil, apperror.BadRequest("approval payload missing rule_id or target_status")
	}
	if targetStatus != "active" && targetStatus != "paused" {
		return nil, apperror.BadRequest("target_status must be active or paused")
	}

	// Bypass the approval gate by calling repo.Update directly (which is what
	// Update does for non-toggle fields). We rely on the repo to record the
	// change log under the checker's name — this keeps the audit trail honest
	// (the approver committed the change, not the requester).
	if _, err := u.repo.Update(ctx, workspaceID, ruleID, map[string]interface{}{"status": targetStatus}, checkerEmail); err != nil {
		return nil, fmt.Errorf("automationRule.ApplyToggleStatus update: %w", err)
	}

	if err := u.approvalRepo.UpdateStatus(ctx, workspaceID, ar.ID, entity.ApprovalStatusApproved, checkerEmail, ""); err != nil {
		return nil, fmt.Errorf("mark approval approved: %w", err)
	}

	updated, err := u.repo.GetByID(ctx, workspaceID, ruleID)
	if err != nil {
		return nil, fmt.Errorf("automationRule.ApplyToggleStatus re-fetch: %w", err)
	}
	return updated, nil
}
