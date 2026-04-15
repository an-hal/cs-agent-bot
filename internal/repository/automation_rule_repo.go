package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// AutomationRuleRepository provides data access for automation_rules and
// rule_change_logs (append-only).
type AutomationRuleRepository interface {
	List(ctx context.Context, workspaceID string, filter entity.AutomationRuleFilter) ([]entity.AutomationRule, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.AutomationRule, error)
	GetByTriggerID(ctx context.Context, workspaceID, triggerID string) (*entity.AutomationRule, error)
	GetActiveByRole(ctx context.Context, workspaceID string, role entity.RuleRole) ([]entity.AutomationRule, error)
	Create(ctx context.Context, r *entity.AutomationRule) error
	// Update applies field-level changes and appends change log entries atomically.
	Update(ctx context.Context, workspaceID, id string, fields map[string]interface{}, editor string) ([]entity.RuleChangeLog, error)
	Delete(ctx context.Context, workspaceID, id string) error

	// Change logs
	ListChangeLogs(ctx context.Context, workspaceID string, limit int) ([]entity.RuleChangeLogWithCode, error)
	ListChangeLogsForRule(ctx context.Context, ruleID string) ([]entity.RuleChangeLog, error)
}

type automationRuleRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewAutomationRuleRepo constructs an AutomationRuleRepository.
func NewAutomationRuleRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) AutomationRuleRepository {
	return &automationRuleRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *automationRuleRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const arCols = `id::text, workspace_id::text, rule_code, trigger_id, template_id,
  role, phase, phase_label, priority, timing, condition, stop_if, sent_flag, channel,
  status, updated_at, updated_by, created_at`

func scanAutomationRule(s rowScanner) (*entity.AutomationRule, error) {
	var r entity.AutomationRule
	err := s.Scan(
		&r.ID, &r.WorkspaceID, &r.RuleCode, &r.TriggerID, &r.TemplateID,
		&r.Role, &r.Phase, &r.PhaseLabel, &r.Priority,
		&r.Timing, &r.Condition, &r.StopIf, &r.SentFlag, &r.Channel,
		&r.Status, &r.UpdatedAt, &r.UpdatedBy, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ─── List ────────────────────────────────────────────────────────────────────

func (r *automationRuleRepo) List(ctx context.Context, workspaceID string, filter entity.AutomationRuleFilter) ([]entity.AutomationRule, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := fmt.Sprintf("SELECT %s FROM automation_rules WHERE workspace_id = $1", arCols)
	args := []interface{}{workspaceID}

	if filter.Role != "" {
		args = append(args, filter.Role)
		q += fmt.Sprintf(" AND role = $%d", len(args))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		q += fmt.Sprintf(" AND status = $%d", len(args))
	}
	if filter.Phase != "" {
		args = append(args, filter.Phase)
		q += fmt.Sprintf(" AND phase = $%d", len(args))
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		q += fmt.Sprintf(" AND (trigger_id ILIKE $%d OR template_id ILIKE $%d OR condition ILIKE $%d OR rule_code ILIKE $%d)",
			len(args), len(args), len(args), len(args))
	}
	q += " ORDER BY phase, created_at"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("automationRule.List: %w", err)
	}
	defer rows.Close()

	var result []entity.AutomationRule
	for rows.Next() {
		rule, err := scanAutomationRule(rows)
		if err != nil {
			return nil, fmt.Errorf("automationRule.List scan: %w", err)
		}
		result = append(result, *rule)
	}
	return result, rows.Err()
}

// ─── GetByID ──────────────────────────────────────────────────────────────────

func (r *automationRuleRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.AutomationRule, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	row := r.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM automation_rules WHERE workspace_id = $1 AND id = $2", arCols),
		workspaceID, id)
	rule, err := scanAutomationRule(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("automationRule.GetByID: %w", err)
	}
	return rule, nil
}

// ─── GetByTriggerID ───────────────────────────────────────────────────────────

func (r *automationRuleRepo) GetByTriggerID(ctx context.Context, workspaceID, triggerID string) (*entity.AutomationRule, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	row := r.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT %s FROM automation_rules WHERE workspace_id = $1 AND trigger_id = $2 AND status = 'active'", arCols),
		workspaceID, triggerID)
	rule, err := scanAutomationRule(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("automationRule.GetByTriggerID: %w", err)
	}
	return rule, nil
}

// ─── GetActiveByRole ──────────────────────────────────────────────────────────

func (r *automationRuleRepo) GetActiveByRole(ctx context.Context, workspaceID string, role entity.RuleRole) ([]entity.AutomationRule, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf("SELECT %s FROM automation_rules WHERE workspace_id = $1 AND role = $2 AND status = 'active' ORDER BY phase, created_at", arCols),
		workspaceID, role)
	if err != nil {
		return nil, fmt.Errorf("automationRule.GetActiveByRole: %w", err)
	}
	defer rows.Close()

	var result []entity.AutomationRule
	for rows.Next() {
		rule, err := scanAutomationRule(rows)
		if err != nil {
			return nil, fmt.Errorf("automationRule.GetActiveByRole scan: %w", err)
		}
		result = append(result, *rule)
	}
	return result, rows.Err()
}

// ─── Create ───────────────────────────────────────────────────────────────────

func (r *automationRuleRepo) Create(ctx context.Context, rule *entity.AutomationRule) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	err := r.db.QueryRowContext(ctx,
		`INSERT INTO automation_rules
         (workspace_id, rule_code, trigger_id, template_id, role, phase, phase_label,
          priority, timing, condition, stop_if, sent_flag, channel, status, updated_by)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
         RETURNING id::text, created_at`,
		rule.WorkspaceID, rule.RuleCode, rule.TriggerID, rule.TemplateID,
		rule.Role, rule.Phase, rule.PhaseLabel, rule.Priority,
		rule.Timing, rule.Condition, rule.StopIf, rule.SentFlag, rule.Channel,
		rule.Status, rule.UpdatedBy,
	).Scan(&rule.ID, &rule.CreatedAt)
	if err != nil {
		return fmt.Errorf("automationRule.Create: %w", err)
	}
	return nil
}

// ─── Update ───────────────────────────────────────────────────────────────────

// Update applies patch fields and atomically logs each field change.
// It returns the list of RuleChangeLog rows that were inserted.
func (r *automationRuleRepo) Update(ctx context.Context, workspaceID, id string, fields map[string]interface{}, editor string) ([]entity.RuleChangeLog, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	allowed := map[string]bool{
		"timing": true, "condition": true, "stop_if": true, "sent_flag": true,
		"template_id": true, "channel": true, "status": true,
		"phase": true, "phase_label": true, "priority": true, "role": true,
	}

	// Fetch current state for diff
	current, err := r.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, fmt.Errorf("automationRule.Update: rule %s not found", id)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("automationRule.Update begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var setClauses []string
	var args []interface{}
	var logs []entity.RuleChangeLog

	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		args = append(args, v)
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, len(args)))

		oldVal := currentFieldString(current, k)
		newVal := fmt.Sprint(v)
		if oldVal == newVal {
			continue
		}
		logs = append(logs, entity.RuleChangeLog{
			RuleID:      id,
			WorkspaceID: workspaceID,
			Field:       k,
			OldValue:    &oldVal,
			NewValue:    newVal,
			EditedBy:    editor,
		})
	}

	if len(setClauses) > 0 {
		// Build args: field values, then editor, id, workspaceID
		execArgs := make([]interface{}, 0, len(args)+3)
		execArgs = append(execArgs, args...)
		execArgs = append(execArgs, editor, id, workspaceID)
		n := len(execArgs)
		q := fmt.Sprintf("UPDATE automation_rules SET %s, updated_at = NOW(), updated_by = $%d WHERE id = $%d AND workspace_id = $%d",
			strings.Join(setClauses, ", "), n-2, n-1, n)
		if _, err := tx.ExecContext(ctx, q, execArgs...); err != nil {
			return nil, fmt.Errorf("automationRule.Update exec: %w", err)
		}
	}

	for i := range logs {
		err := tx.QueryRowContext(ctx,
			`INSERT INTO rule_change_logs (rule_id, workspace_id, field, old_value, new_value, edited_by)
             VALUES ($1,$2,$3,$4,$5,$6) RETURNING id::text, edited_at`,
			logs[i].RuleID, logs[i].WorkspaceID, logs[i].Field,
			logs[i].OldValue, logs[i].NewValue, logs[i].EditedBy,
		).Scan(&logs[i].ID, &logs[i].EditedAt)
		if err != nil {
			return nil, fmt.Errorf("automationRule.Update insert log: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("automationRule.Update commit: %w", err)
	}
	return logs, nil
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (r *automationRuleRepo) Delete(ctx context.Context, workspaceID, id string) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx,
		`DELETE FROM automation_rules WHERE workspace_id = $1 AND id = $2`, workspaceID, id)
	if err != nil {
		return fmt.Errorf("automationRule.Delete: %w", err)
	}
	return nil
}

// ─── ChangeLogs ───────────────────────────────────────────────────────────────

func (r *automationRuleRepo) ListChangeLogs(ctx context.Context, workspaceID string, limit int) ([]entity.RuleChangeLogWithCode, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT cl.id::text, cl.rule_id::text, cl.workspace_id::text, cl.field,
                cl.old_value, cl.new_value, cl.edited_by, cl.edited_at,
                ar.rule_code
         FROM rule_change_logs cl
         JOIN automation_rules ar ON ar.id = cl.rule_id
         WHERE cl.workspace_id = $1
         ORDER BY cl.edited_at DESC LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("automationRule.ListChangeLogs: %w", err)
	}
	defer rows.Close()

	var result []entity.RuleChangeLogWithCode
	for rows.Next() {
		var l entity.RuleChangeLogWithCode
		if err := rows.Scan(
			&l.ID, &l.RuleID, &l.WorkspaceID, &l.Field,
			&l.OldValue, &l.NewValue, &l.EditedBy, &l.EditedAt,
			&l.RuleCode,
		); err != nil {
			return nil, fmt.Errorf("automationRule.ListChangeLogs scan: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

func (r *automationRuleRepo) ListChangeLogsForRule(ctx context.Context, ruleID string) ([]entity.RuleChangeLog, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.db.QueryContext(ctx,
		`SELECT id::text, rule_id::text, workspace_id::text, field,
                old_value, new_value, edited_by, edited_at
         FROM rule_change_logs WHERE rule_id = $1 ORDER BY edited_at DESC`, ruleID)
	if err != nil {
		return nil, fmt.Errorf("automationRule.ListChangeLogsForRule: %w", err)
	}
	defer rows.Close()

	var result []entity.RuleChangeLog
	for rows.Next() {
		var l entity.RuleChangeLog
		if err := rows.Scan(
			&l.ID, &l.RuleID, &l.WorkspaceID, &l.Field,
			&l.OldValue, &l.NewValue, &l.EditedBy, &l.EditedAt,
		); err != nil {
			return nil, fmt.Errorf("automationRule.ListChangeLogsForRule scan: %w", err)
		}
		result = append(result, l)
	}
	return result, rows.Err()
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// currentFieldString returns the string representation of a field on the
// current AutomationRule for change-log diffing.
func currentFieldString(r *entity.AutomationRule, field string) string {
	switch field {
	case "timing":
		return r.Timing
	case "condition":
		return r.Condition
	case "stop_if":
		return r.StopIf
	case "sent_flag":
		if r.SentFlag != nil {
			return *r.SentFlag
		}
		return ""
	case "template_id":
		if r.TemplateID != nil {
			return *r.TemplateID
		}
		return ""
	case "channel":
		return string(r.Channel)
	case "status":
		return string(r.Status)
	case "phase":
		return r.Phase
	case "phase_label":
		if r.PhaseLabel != nil {
			return *r.PhaseLabel
		}
		return ""
	case "priority":
		if r.Priority != nil {
			return *r.Priority
		}
		return ""
	case "role":
		return string(r.Role)
	default:
		return ""
	}
}
