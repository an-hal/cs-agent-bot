package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// TriggerRuleRepository provides data access for the trigger_rules table.
type TriggerRuleRepository interface {
	GetActiveRulesOrdered(ctx context.Context) ([]entity.TriggerRule, error)
	GetByID(ctx context.Context, ruleID string) (*entity.TriggerRule, error)
	GetAllPaginated(ctx context.Context, filter entity.TriggerRuleFilter, p pagination.Params) ([]entity.TriggerRule, int64, error)
	Create(ctx context.Context, rule entity.TriggerRule) error
	Update(ctx context.Context, ruleID string, fields map[string]interface{}) error
	Delete(ctx context.Context, ruleID string) error
}

type triggerRuleRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewTriggerRuleRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) TriggerRuleRepository {
	return &triggerRuleRepo{
		db:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *triggerRuleRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

var triggerRuleColumns = []string{
	"rule_id", "rule_group", "priority", "sub_priority",
	"condition", "action_type", "template_id", "flag_key",
	"escalation_id", "esc_priority", "esc_reason", "extra_flags",
	"stop_on_fire", "active", "description", "workspace_id",
	"created_at", "updated_at",
}

func scanTriggerRule(scanner interface{ Scan(dest ...interface{}) error }) (*entity.TriggerRule, error) {
	var r entity.TriggerRule
	var conditionBytes, extraFlagsBytes []byte
	err := scanner.Scan(
		&r.RuleID, &r.RuleGroup, &r.Priority, &r.SubPriority,
		&conditionBytes, &r.ActionType, &r.TemplateID, &r.FlagKey,
		&r.EscalationID, &r.EscPriority, &r.EscReason, &extraFlagsBytes,
		&r.StopOnFire, &r.Active, &r.Description, &r.WorkspaceID,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	r.Condition = json.RawMessage(conditionBytes)
	if extraFlagsBytes != nil {
		r.ExtraFlags = json.RawMessage(extraFlagsBytes)
	}
	return &r, nil
}

// GetActiveRulesOrdered returns all active rules sorted by priority, sub_priority.
func (r *triggerRuleRepo) GetActiveRulesOrdered(ctx context.Context) ([]entity.TriggerRule, error) {
	ctx, span := r.tracer.Start(ctx, "trigger_rule.repository.GetActiveRulesOrdered")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(triggerRuleColumns...).
		From("trigger_rules").
		Where(sq.Eq{"active": true}).
		OrderBy("priority ASC", "sub_priority ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetActiveRulesOrdered: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query GetActiveRulesOrdered: %w", err)
	}
	defer rows.Close()

	var rules []entity.TriggerRule
	for rows.Next() {
		rule, err := scanTriggerRule(rows)
		if err != nil {
			return nil, fmt.Errorf("scan trigger rule: %w", err)
		}
		rules = append(rules, *rule)
	}
	return rules, rows.Err()
}

// GetByID returns a single trigger rule by rule_id.
func (r *triggerRuleRepo) GetByID(ctx context.Context, ruleID string) (*entity.TriggerRule, error) {
	ctx, span := r.tracer.Start(ctx, "trigger_rule.repository.GetByID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(triggerRuleColumns...).
		From("trigger_rules").
		Where(sq.Eq{"rule_id": ruleID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetByID: %w", err)
	}

	rule, err := scanTriggerRule(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query GetByID: %w", err)
	}
	return rule, nil
}

// GetAllPaginated returns paginated trigger rules with optional filters.
func (r *triggerRuleRepo) GetAllPaginated(ctx context.Context, filter entity.TriggerRuleFilter, p pagination.Params) ([]entity.TriggerRule, int64, error) {
	ctx, span := r.tracer.Start(ctx, "trigger_rule.repository.GetAllPaginated")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	base := database.PSQL.Select(triggerRuleColumns...).From("trigger_rules")
	countBase := database.PSQL.Select("COUNT(*)").From("trigger_rules")

	if filter.RuleGroup != "" {
		base = base.Where(sq.Eq{"rule_group": filter.RuleGroup})
		countBase = countBase.Where(sq.Eq{"rule_group": filter.RuleGroup})
	}
	if filter.ActionType != "" {
		base = base.Where(sq.Eq{"action_type": filter.ActionType})
		countBase = countBase.Where(sq.Eq{"action_type": filter.ActionType})
	}
	if filter.Active != nil {
		base = base.Where(sq.Eq{"active": *filter.Active})
		countBase = countBase.Where(sq.Eq{"active": *filter.Active})
	}
	if filter.WorkspaceID != "" {
		base = base.Where(sq.Eq{"workspace_id": filter.WorkspaceID})
		countBase = countBase.Where(sq.Eq{"workspace_id": filter.WorkspaceID})
	}
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		or := sq.Or{
			sq.ILike{"rule_id": like},
			sq.ILike{"rule_group": like},
			sq.ILike{"description": like},
		}
		base = base.Where(or)
		countBase = countBase.Where(or)
	}

	// Count
	countQuery, countArgs, err := countBase.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count trigger rules: %w", err)
	}

	// Data
	query, args, err := base.
		OrderBy("priority ASC", "sub_priority ASC").
		Limit(uint64(p.Limit)).
		Offset(uint64(p.Offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query trigger rules: %w", err)
	}
	defer rows.Close()

	var rules []entity.TriggerRule
	for rows.Next() {
		rule, err := scanTriggerRule(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan trigger rule: %w", err)
		}
		rules = append(rules, *rule)
	}
	return rules, total, rows.Err()
}

// Create inserts a new trigger rule.
func (r *triggerRuleRepo) Create(ctx context.Context, rule entity.TriggerRule) error {
	ctx, span := r.tracer.Start(ctx, "trigger_rule.repository.Create")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Insert("trigger_rules").
		Columns(triggerRuleColumns...).
		Values(
			rule.RuleID, rule.RuleGroup, rule.Priority, rule.SubPriority,
			rule.Condition, rule.ActionType, rule.TemplateID, rule.FlagKey,
			rule.EscalationID, rule.EscPriority, rule.EscReason, rule.ExtraFlags,
			rule.StopOnFire, rule.Active, rule.Description, rule.WorkspaceID,
			rule.CreatedAt, rule.UpdatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query Create: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert trigger rule: %w", err)
	}
	return nil
}

// Update partially updates a trigger rule by rule_id.
func (r *triggerRuleRepo) Update(ctx context.Context, ruleID string, fields map[string]interface{}) error {
	ctx, span := r.tracer.Start(ctx, "trigger_rule.repository.Update")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	fields["updated_at"] = time.Now()

	query, args, err := database.PSQL.
		Update("trigger_rules").
		SetMap(fields).
		Where(sq.Eq{"rule_id": ruleID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query Update: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update trigger rule: %w", err)
	}
	return nil
}

// Delete soft-deletes a trigger rule by setting active=false.
func (r *triggerRuleRepo) Delete(ctx context.Context, ruleID string) error {
	return r.Update(ctx, ruleID, map[string]interface{}{"active": false})
}
