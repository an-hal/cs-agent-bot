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
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

type LogRepository interface {
	AppendLog(ctx context.Context, entry entity.ActionLog) error
	SentTodayAlready(ctx context.Context, companyID string) (bool, error)
	MessageIDExists(ctx context.Context, messageID string) (bool, error)
	AppendActivity(ctx context.Context, entry entity.ActivityLog) error
	GetActivities(ctx context.Context, filter entity.ActivityFilter) ([]entity.ActivityLog, int, error)
	GetActivityStats(ctx context.Context, workspaceIDs []string) (entity.ActivityStats, error)
	GetRecentActivities(ctx context.Context, workspaceIDs []string, since time.Time, limit int) ([]entity.ActivityLog, error)
	GetCompanySummary(ctx context.Context, workspaceIDs []string, companyID string) (*entity.CompanySummary, error)

	// Bot-action specific reads (dashboard bot-activity sidebar).
	GetRecentActionLogs(ctx context.Context, workspaceIDs []string, limit int) ([]entity.ActionLog, error)
	GetActionLogSummary(ctx context.Context, workspaceIDs []string, since time.Time) (*entity.ActionLogSummary, error)
	GetTodayActionLogs(ctx context.Context, workspaceIDs []string, limit int) ([]entity.ActionLog, error)
}

type logRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewLogRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) LogRepository {
	return &logRepo{
		db:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *logRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// AppendLog inserts a new action log entry into the action_log table.
func (r *logRepo) AppendLog(ctx context.Context, entry entity.ActionLog) error {
	ctx, span := r.tracer.Start(ctx, "log.repository.AppendLog")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	status := "N"
	if entry.MessageSent {
		status = "Y"
	}

	query, args, err := database.PSQL.
		Insert("action_log").
		Columns(
			"triggered_at",
			"company_id",
			"company_name",
			"trigger_type",
			"template_id",
			"channel",
			"message_sent",
			"status",
			"response_classification",
			"next_action_triggered",
			"log_notes",
		).
		Values(
			entry.Timestamp,
			entry.CompanyID,
			entry.CompanyName,
			entry.TriggerType,
			entry.TemplateID,
			entry.Channel,
			entry.MessageSent,
			status,
			entry.ResponseClassification,
			entry.NextActionTriggered,
			entry.LogNotes,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query AppendLog: %w", err)
	}

	if _, err = r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert action log: %w", err)
	}

	return nil
}

// SentTodayAlready checks whether a WhatsApp message was already sent to the
// given company today by querying the action_log table.
func (r *logRepo) SentTodayAlready(ctx context.Context, companyID string) (bool, error) {
	ctx, span := r.tracer.Start(ctx, "log.repository.SentTodayAlready")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("1").
		From("action_log").
		Where(sq.And{
			sq.Eq{"company_id": companyID},
			sq.Eq{"channel": entity.ChannelWhatsApp},
			sq.Eq{"message_sent": true},
			sq.Expr("triggered_at::date = CURRENT_DATE"),
		}).
		Limit(1).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build query SentTodayAlready: %w", err)
	}

	var dummy int
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&dummy)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("query SentTodayAlready: %w", err)
	}

	return true, nil
}

// AppendActivity inserts a new audit trail entry into the activity_log table.
func (r *logRepo) AppendActivity(ctx context.Context, entry entity.ActivityLog) error {
	ctx, span := r.tracer.Start(ctx, "log.repository.AppendActivity")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = time.Now()
	}

	query, args, err := database.PSQL.
		Insert("activity_log").
		Columns(
			"workspace_id",
			"category",
			"actor_type",
			"actor",
			"action",
			"target",
			"detail",
			"ref_id",
			"resource_type",
			"status",
			"occurred_at",
		).
		Values(
			entry.WorkspaceID,
			entry.Category,
			entry.ActorType,
			entry.Actor,
			entry.Action,
			entry.Target,
			entry.Detail,
			entry.RefID,
			entry.ResourceType,
			entry.Status,
			entry.OccurredAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query AppendActivity: %w", err)
	}

	if _, err = r.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert activity log: %w", err)
	}

	return nil
}

// GetActivities returns paginated activity log entries matching the given filter,
// along with the total count of matching rows.
func (r *logRepo) GetActivities(ctx context.Context, filter entity.ActivityFilter) ([]entity.ActivityLog, int, error) {
	ctx, span := r.tracer.Start(ctx, "log.repository.GetActivities")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	// Build WHERE conditions
	cond := sq.And{}
	if len(filter.WorkspaceIDs) > 0 {
		cond = append(cond, sq.Expr("workspace_id = ANY(?)", pq.Array(filter.WorkspaceIDs)))
	}
	if filter.Category != "" {
		cond = append(cond, sq.Eq{"category": filter.Category})
	}
	if filter.ResourceType != "" {
		cond = append(cond, sq.Eq{"resource_type": filter.ResourceType})
	}
	if filter.RefID != "" {
		cond = append(cond, sq.Eq{"ref_id": filter.RefID})
	}
	if filter.Since != nil {
		cond = append(cond, sq.GtOrEq{"occurred_at": *filter.Since})
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	// COUNT query
	countQuery, countArgs, err := database.PSQL.
		Select("COUNT(*)").
		From("activity_log").
		Where(cond).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query GetActivities: %w", err)
	}

	var total int
	if err = r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count activities: %w", err)
	}

	// Data query
	dataQuery, dataArgs, err := database.PSQL.
		Select(
			"id", "workspace_id", "category", "actor_type",
			"actor", "action", "target", "detail",
			"ref_id", "resource_type", "status", "occurred_at",
		).
		From("activity_log").
		Where(cond).
		OrderBy("occurred_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(filter.Offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build data query GetActivities: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query activities: %w", err)
	}
	defer rows.Close()

	var logs []entity.ActivityLog
	for rows.Next() {
		var a entity.ActivityLog
		var (
			nsWorkspaceID  sql.NullString
			nsActor        sql.NullString
			nsTarget       sql.NullString
			nsDetail       sql.NullString
			nsRefID        sql.NullString
			nsResourceType sql.NullString
			nsStatus       sql.NullString
		)
		if err := rows.Scan(
			&a.ID, &nsWorkspaceID, &a.Category, &a.ActorType,
			&nsActor, &a.Action, &nsTarget, &nsDetail,
			&nsRefID, &nsResourceType, &nsStatus, &a.OccurredAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan activity row: %w", err)
		}

		a.WorkspaceID = nsWorkspaceID.String
		a.Actor = nsActor.String
		a.Target = nsTarget.String
		a.Detail = nsDetail.String
		a.RefID = nsRefID.String
		a.ResourceType = nsResourceType.String
		a.Status = nsStatus.String

		logs = append(logs, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate activity rows: %w", err)
	}

	return logs, total, nil
}

// GetActivityStats returns the 7 stat counters for the given workspace IDs.
func (r *logRepo) GetActivityStats(ctx context.Context, workspaceIDs []string) (entity.ActivityStats, error) {
	ctx, span := r.tracer.Start(ctx, "log.repository.GetActivityStats")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	wsCond := sq.Expr("workspace_id = ANY(?)", pq.Array(workspaceIDs))

	query, args, err := database.PSQL.
		Select(
			"COUNT(*)",
			"COUNT(*) FILTER (WHERE occurred_at::date = CURRENT_DATE)",
			"COUNT(*) FILTER (WHERE category = 'bot')",
			"COUNT(*) FILTER (WHERE category IN ('data','team'))",
			"COUNT(*) FILTER (WHERE category = 'data')",
			"COUNT(*) FILTER (WHERE category = 'team')",
			"0",
		).
		From("activity_log").
		Where(wsCond).
		ToSql()
	if err != nil {
		return entity.ActivityStats{}, fmt.Errorf("build query GetActivityStats: %w", err)
	}

	var stats entity.ActivityStats
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.Total, &stats.Today, &stats.Bot, &stats.Human,
		&stats.DataMutations, &stats.TeamActions, &stats.Escalations,
	); err != nil {
		return entity.ActivityStats{}, fmt.Errorf("query GetActivityStats: %w", err)
	}

	// Escalation count from the escalations table (status = Open).
	escQuery, escArgs, err := database.PSQL.
		Select("COUNT(*)").
		From("escalations").
		Where(sq.And{
			sq.Expr("workspace_id::text = ANY(?)", pq.Array(workspaceIDs)),
			sq.Eq{"status": entity.EscalationStatusOpen},
		}).
		ToSql()
	if err != nil {
		return stats, fmt.Errorf("build escalation count query: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, escQuery, escArgs...).Scan(&stats.Escalations); err != nil {
		return stats, fmt.Errorf("query escalation count: %w", err)
	}

	return stats, nil
}

// GetRecentActivities returns activity log entries since the given timestamp, ordered desc.
func (r *logRepo) GetRecentActivities(ctx context.Context, workspaceIDs []string, since time.Time, limit int) ([]entity.ActivityLog, error) {
	ctx, span := r.tracer.Start(ctx, "log.repository.GetRecentActivities")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	query, args, err := database.PSQL.
		Select(
			"id", "workspace_id", "category", "actor_type",
			"actor", "action", "target", "detail",
			"ref_id", "resource_type", "status", "occurred_at",
		).
		From("activity_log").
		Where(sq.And{
			sq.Expr("workspace_id = ANY(?)", pq.Array(workspaceIDs)),
			sq.GtOrEq{"occurred_at": since},
		}).
		OrderBy("occurred_at DESC").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetRecentActivities: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query GetRecentActivities: %w", err)
	}
	defer rows.Close()

	var logs []entity.ActivityLog
	for rows.Next() {
		var a entity.ActivityLog
		var (
			nsWorkspaceID  sql.NullString
			nsActor        sql.NullString
			nsTarget       sql.NullString
			nsDetail       sql.NullString
			nsRefID        sql.NullString
			nsResourceType sql.NullString
			nsStatus       sql.NullString
		)
		if err := rows.Scan(
			&a.ID, &nsWorkspaceID, &a.Category, &a.ActorType,
			&nsActor, &a.Action, &nsTarget, &nsDetail,
			&nsRefID, &nsResourceType, &nsStatus, &a.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("scan recent activity row: %w", err)
		}
		a.WorkspaceID = nsWorkspaceID.String
		a.Actor = nsActor.String
		a.Target = nsTarget.String
		a.Detail = nsDetail.String
		a.RefID = nsRefID.String
		a.ResourceType = nsResourceType.String
		a.Status = nsStatus.String
		logs = append(logs, a)
	}
	return logs, rows.Err()
}

// GetCompanySummary returns aggregated action log stats for a single company.
func (r *logRepo) GetCompanySummary(ctx context.Context, workspaceIDs []string, companyID string) (*entity.CompanySummary, error) {
	ctx, span := r.tracer.Start(ctx, "log.repository.GetCompanySummary")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := `
		SELECT
			COALESCE(company_id, $1) AS company_id,
			COALESCE(MAX(company_name), '') AS company_name,
			COUNT(*) AS total_sent,
			COUNT(*) FILTER (WHERE replied = true) AS total_replied,
			CASE WHEN COUNT(*) > 0
				THEN ROUND(COUNT(*) FILTER (WHERE replied = true)::numeric / COUNT(*)::numeric * 100, 1)
				ELSE 0
			END AS reply_rate,
			COALESCE(MAX(occurred_at)::text, '') AS last_sent_at,
			COALESCE(
				(SELECT trigger_id FROM activity_log WHERE company_id = $1 AND workspace_id = ANY($2) ORDER BY occurred_at DESC LIMIT 1),
				''
			) AS last_trigger_id,
			COALESCE(
				(SELECT status FROM activity_log WHERE company_id = $1 AND workspace_id = ANY($2) ORDER BY occurred_at DESC LIMIT 1),
				''
			) AS last_status,
			COALESCE(
				(SELECT phase FROM activity_log WHERE company_id = $1 AND workspace_id = ANY($2) ORDER BY occurred_at DESC LIMIT 1),
				''
			) AS current_phase
		FROM activity_log
		WHERE company_id = $1
			AND workspace_id = ANY($2)
			AND category = 'bot'
	`

	var s entity.CompanySummary
	if err := r.db.QueryRowContext(ctx, query, companyID, pq.Array(workspaceIDs)).Scan(
		&s.CompanyID, &s.CompanyName, &s.TotalSent, &s.TotalReplied,
		&s.ReplyRate, &s.LastSentAt, &s.LastTriggerID, &s.LastStatus, &s.CurrentPhase,
	); err != nil {
		if err == sql.ErrNoRows {
			return &entity.CompanySummary{CompanyID: companyID}, nil
		}
		return nil, fmt.Errorf("query GetCompanySummary: %w", err)
	}

	// Check for active escalation
	escQuery, escArgs, escErr := database.PSQL.
		Select("1").
		From("escalations").
		Where(sq.And{
			sq.Eq{"company_id": companyID},
			sq.Eq{"status": entity.EscalationStatusOpen},
		}).
		Limit(1).
		ToSql()
	if escErr != nil {
		return &s, nil
	}
	var dummy int
	if err := r.db.QueryRowContext(ctx, escQuery, escArgs...).Scan(&dummy); err == nil {
		s.HasActiveEscalation = true
	}

	return &s, nil
}

// MessageIDExists checks whether the given message ID already exists in the
// action_log table. Returns false with no error when messageID is empty.
func (r *logRepo) MessageIDExists(ctx context.Context, messageID string) (bool, error) {
	if messageID == "" {
		return false, nil
	}

	ctx, span := r.tracer.Start(ctx, "log.repository.MessageIDExists")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("1").
		From("action_log").
		Where(sq.Eq{"message_id": messageID}).
		Limit(1).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build query MessageIDExists: %w", err)
	}

	var dummy int
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&dummy)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("query MessageIDExists: %w", err)
	}

	return true, nil
}
