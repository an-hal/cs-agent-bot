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
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type EscalationRepository interface {
	GetOpenByCompanyAndEscID(ctx context.Context, companyID, escID string) (*entity.Escalation, error)
	OpenEscalation(ctx context.Context, esc entity.Escalation) error
}

type escalationRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewEscalationRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) EscalationRepository {
	return &escalationRepo{
		db:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *escalationRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// escalationColumns lists every column read from the escalations table in scan order.
const escalationColumns = "id, esc_id, company_id, status, triggered_at, priority, trigger_condition, COALESCE(notified_party, '') as notified_party, COALESCE(telegram_message_sent, '') as telegram_message_sent, resolved_at, COALESCE(resolved_by, '') as resolved_by, COALESCE(notes, '') as notes, COALESCE(workspace_id::text, '') as workspace_id"

// GetOpenByCompanyAndEscID returns an open escalation matching the given company
// and escalation rule ID. Returns nil, nil when no matching open escalation exists.
func (r *escalationRepo) GetOpenByCompanyAndEscID(ctx context.Context, companyID, escID string) (*entity.Escalation, error) {
	ctx, span := r.tracer.Start(ctx, "escalation.repository.GetOpenByCompanyAndEscID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(escalationColumns).
		From("escalations").
		Where(sq.And{
			sq.Eq{"company_id": companyID},
			sq.Eq{"esc_id": escID},
			sq.Eq{"status": entity.EscalationStatusOpen},
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetOpenByCompanyAndEscID: %w", err)
	}

	var esc entity.Escalation
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&esc.EscalationID,
		&esc.EscID,
		&esc.CompanyID,
		&esc.Status,
		&esc.CreatedAt,
		&esc.Priority,
		&esc.Reason,
		&esc.NotifiedParty,
		&esc.TelegramMessageSent,
		&esc.ResolvedAt,
		&esc.ResolvedBy,
		&esc.EscNotes,
		&esc.WorkspaceID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query GetOpenByCompanyAndEscID: %w", err)
	}

	return &esc, nil
}

// OpenEscalation inserts a new escalation record with status "Open".
// If EscalationID is empty, a new UUID is generated. CreatedAt is set to NOW().
func (r *escalationRepo) OpenEscalation(ctx context.Context, esc entity.Escalation) error {
	ctx, span := r.tracer.Start(ctx, "escalation.repository.OpenEscalation")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if esc.EscalationID == "" {
		esc.EscalationID = uuid.New().String()
	}
	esc.Status = entity.EscalationStatusOpen
	esc.CreatedAt = time.Now()

	query, args, err := database.PSQL.
		Insert("escalations").
		Columns(
			"esc_id",
			"company_id",
			"trigger_condition",
			"priority",
			"status",
			"triggered_at",
		).
		Values(
			esc.EscalationID,
			esc.CompanyID,
			esc.Reason,
			esc.Priority,
			esc.Status,
			esc.CreatedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query OpenEscalation: %w", err)
	}

	if _, err = r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert escalation: %w", err)
	}

	return nil
}
