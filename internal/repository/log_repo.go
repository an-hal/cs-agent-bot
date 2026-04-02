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

type LogRepository interface {
	AppendLog(ctx context.Context, entry entity.ActionLog) error
	SentTodayAlready(ctx context.Context, companyID string) (bool, error)
	MessageIDExists(ctx context.Context, messageID string) (bool, error)
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
