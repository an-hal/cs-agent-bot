package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

type EscalationRepository interface {
	GetOpenByCompanyAndEscID(ctx context.Context, companyID, escID string) (*entity.Escalation, error)
	GetByCompanyID(ctx context.Context, companyID string) ([]entity.Escalation, error)
	GetAllPaginated(ctx context.Context, filter entity.EscalationFilter, p pagination.Params) ([]entity.Escalation, int64, error)
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

// GetByCompanyID returns all escalations for a given company, newest first.
func (r *escalationRepo) GetByCompanyID(ctx context.Context, companyID string) ([]entity.Escalation, error) {
	ctx, span := r.tracer.Start(ctx, "escalation.repository.GetByCompanyID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(escalationColumns).
		From("escalations").
		Where(sq.Eq{"company_id": companyID}).
		OrderBy("triggered_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetByCompanyID: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query GetByCompanyID: %w", err)
	}
	defer rows.Close()

	var escalations []entity.Escalation
	for rows.Next() {
		var esc entity.Escalation
		if err := rows.Scan(
			&esc.EscalationID, &esc.EscID, &esc.CompanyID, &esc.Status, &esc.CreatedAt,
			&esc.Priority, &esc.Reason, &esc.NotifiedParty, &esc.TelegramMessageSent,
			&esc.ResolvedAt, &esc.ResolvedBy, &esc.EscNotes, &esc.WorkspaceID,
		); err != nil {
			return nil, fmt.Errorf("scan escalation: %w", err)
		}
		escalations = append(escalations, esc)
	}
	return escalations, rows.Err()
}

// GetAllPaginated returns paginated escalations with optional filters.
func (r *escalationRepo) GetAllPaginated(ctx context.Context, filter entity.EscalationFilter, p pagination.Params) ([]entity.Escalation, int64, error) {
	ctx, span := r.tracer.Start(ctx, "escalation.repository.GetAllPaginated")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	where := sq.And{}
	if len(filter.WorkspaceIDs) > 0 {
		where = append(where, sq.Expr("workspace_id::text = ANY(?)", pq.Array(filter.WorkspaceIDs)))
	}
	if filter.CompanyID != "" {
		where = append(where, sq.Eq{"company_id": filter.CompanyID})
	}
	if filter.Status != "" {
		where = append(where, sq.Eq{"status": filter.Status})
	}
	if filter.Priority != "" {
		where = append(where, sq.Eq{"priority": filter.Priority})
	}
	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		where = append(where, sq.Or{
			sq.ILike{"company_id": pattern},
			sq.ILike{"trigger_condition": pattern},
			sq.ILike{"esc_id": pattern},
			sq.ILike{"notes": pattern},
		})
	}

	// Count
	countBuilder := database.PSQL.Select("COUNT(*)").From("escalations")
	if len(where) > 0 {
		countBuilder = countBuilder.Where(where)
	}
	countQ, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	var total int64
	if scanErr := r.db.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); scanErr != nil {
		return nil, 0, fmt.Errorf("count escalations: %w", scanErr)
	}

	// Data
	dataBuilder := database.PSQL.
		Select(escalationColumns).
		From("escalations").
		OrderBy("triggered_at DESC").
		Limit(uint64(p.Limit)).
		Offset(uint64(p.Offset))
	if len(where) > 0 {
		dataBuilder = dataBuilder.Where(where)
	}
	dataQ, dataArgs, err := dataBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build data query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query escalations: %w", err)
	}
	defer rows.Close()

	var escalations []entity.Escalation
	for rows.Next() {
		var esc entity.Escalation
		if err := rows.Scan(
			&esc.EscalationID, &esc.EscID, &esc.CompanyID, &esc.Status, &esc.CreatedAt,
			&esc.Priority, &esc.Reason, &esc.NotifiedParty, &esc.TelegramMessageSent,
			&esc.ResolvedAt, &esc.ResolvedBy, &esc.EscNotes, &esc.WorkspaceID,
		); err != nil {
			return nil, 0, fmt.Errorf("scan escalation: %w", err)
		}
		escalations = append(escalations, esc)
	}
	return escalations, total, rows.Err()
}
