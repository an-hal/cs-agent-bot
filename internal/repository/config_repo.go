package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type ConfigRepository interface {
	GetAllTemplates(ctx context.Context) ([]entity.TriggerTemplate, error)
	GetTemplateByID(ctx context.Context, templateID string) (*entity.TriggerTemplate, error)
	GetEscalationTemplate(ctx context.Context, escID string) (*entity.EscalationTemplate, error)
}

type configRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewConfigRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ConfigRepository {
	return &configRepo{
		db:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *configRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// templateColumns lists the columns read from the templates table.
const templateColumns = "template_id, wa_content, template_category"

// GetAllTemplates returns all active trigger templates ordered by template_id.
func (r *configRepo) GetAllTemplates(ctx context.Context) ([]entity.TriggerTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "config.repository.GetAllTemplates")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(templateColumns).
		From("templates").
		Where(sq.Eq{"active": true}).
		OrderBy("template_id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetAllTemplates: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query GetAllTemplates: %w", err)
	}
	defer rows.Close()

	var templates []entity.TriggerTemplate
	for rows.Next() {
		var t entity.TriggerTemplate
		if err = rows.Scan(&t.TemplateID, &t.Body, &t.TriggerType); err != nil {
			return nil, fmt.Errorf("scan template row: %w", err)
		}
		templates = append(templates, t)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate template rows: %w", err)
	}

	return templates, nil
}

// GetTemplateByID returns a single active trigger template by template_id.
// Returns an error if the template is not found.
func (r *configRepo) GetTemplateByID(ctx context.Context, templateID string) (*entity.TriggerTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "config.repository.GetTemplateByID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(templateColumns).
		From("templates").
		Where(sq.And{
			sq.Eq{"template_id": templateID},
			sq.Eq{"active": true},
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetTemplateByID: %w", err)
	}

	var t entity.TriggerTemplate
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&t.TemplateID, &t.Body, &t.TriggerType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("template not found: %s", templateID)
		}
		return nil, fmt.Errorf("query GetTemplateByID: %w", err)
	}

	return &t, nil
}

// escalationRuleColumns lists the columns read from the escalation_rules table.
const escalationRuleColumns = "esc_id, telegram_msg, name, priority"

// GetEscalationTemplate returns a single active escalation template by esc_id.
// Returns an error if the escalation template is not found.
func (r *configRepo) GetEscalationTemplate(ctx context.Context, escID string) (*entity.EscalationTemplate, error) {
	ctx, span := r.tracer.Start(ctx, "config.repository.GetEscalationTemplate")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(escalationRuleColumns).
		From("escalation_rules").
		Where(sq.And{
			sq.Eq{"esc_id": escID},
			sq.Eq{"active": true},
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetEscalationTemplate: %w", err)
	}

	var t entity.EscalationTemplate
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&t.EscID, &t.TelegramMsg, &t.Name, &t.Priority)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("escalation template not found: %s", escID)
		}
		return nil, fmt.Errorf("query GetEscalationTemplate: %w", err)
	}

	return &t, nil
}
