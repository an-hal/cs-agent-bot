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
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

// PaymentLogRepository is an append-only store for invoice payment events.
type PaymentLogRepository interface {
	Append(ctx context.Context, log entity.PaymentLog) error
	AppendTx(ctx context.Context, tx *sql.Tx, log entity.PaymentLog) error
	GetByInvoiceID(ctx context.Context, invoiceID string, limit int) ([]entity.PaymentLog, error)
	GetRecentByWorkspace(ctx context.Context, wsIDs []string, limit int) ([]entity.PaymentLog, error)
}

type paymentLogRepo struct {
	DB           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewPaymentLogRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) PaymentLogRepository {
	return &paymentLogRepo{
		DB:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *paymentLogRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *paymentLogRepo) insert(ctx context.Context, execer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}, log entity.PaymentLog) error {
	rawPayload := []byte("{}")
	if log.RawPayload != nil {
		var err error
		rawPayload, err = json.Marshal(log.RawPayload)
		if err != nil {
			return fmt.Errorf("marshal raw_payload: %w", err)
		}
	}

	query, args, err := database.PSQL.
		Insert("payment_logs").
		Columns(
			"workspace_id", "invoice_id", "event_type",
			"amount_paid", "payment_method", "payment_channel", "payment_ref",
			"old_status", "new_status", "old_stage", "new_stage",
			"actor", "notes", "raw_payload", "timestamp",
		).
		Values(
			log.WorkspaceID, log.InvoiceID, log.EventType,
			log.AmountPaid, log.PaymentMethod, log.PaymentChannel, log.PaymentRef,
			log.OldStatus, log.NewStatus, log.OldStage, log.NewStage,
			log.Actor, log.Notes, string(rawPayload), log.Timestamp,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err = execer.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert payment_log: %w", err)
	}
	return nil
}

// Append writes a payment log entry without a transaction.
func (r *paymentLogRepo) Append(ctx context.Context, log entity.PaymentLog) error {
	ctx, span := r.tracer.Start(ctx, "payment_log.repository.Append")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	return r.insert(ctx, r.DB, log)
}

// AppendTx writes a payment log entry within the supplied transaction.
func (r *paymentLogRepo) AppendTx(ctx context.Context, tx *sql.Tx, log entity.PaymentLog) error {
	ctx, span := r.tracer.Start(ctx, "payment_log.repository.AppendTx")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	return r.insert(ctx, tx, log)
}

// GetByInvoiceID returns payment logs for an invoice, newest first.
func (r *paymentLogRepo) GetByInvoiceID(ctx context.Context, invoiceID string, limit int) ([]entity.PaymentLog, error) {
	ctx, span := r.tracer.Start(ctx, "payment_log.repository.GetByInvoiceID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	builder := database.PSQL.
		Select(
			"id::text", "workspace_id::text", "invoice_id", "event_type",
			"amount_paid", "COALESCE(payment_method,'') AS payment_method",
			"COALESCE(payment_channel,'') AS payment_channel",
			"COALESCE(payment_ref,'') AS payment_ref",
			"COALESCE(old_status,'') AS old_status", "COALESCE(new_status,'') AS new_status",
			"COALESCE(old_stage,'') AS old_stage", "COALESCE(new_stage,'') AS new_stage",
			"COALESCE(actor,'') AS actor", "COALESCE(notes,'') AS notes",
			"COALESCE(raw_payload, '{}'::jsonb) AS raw_payload", "timestamp",
		).
		From("payment_logs").
		Where(sq.Eq{"invoice_id": invoiceID}).
		OrderBy("timestamp DESC")

	if limit > 0 {
		builder = builder.Limit(uint64(limit))
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	return r.scanLogs(ctx, query, args...)
}

// GetRecentByWorkspace returns the most recent payment log entries across workspaces.
func (r *paymentLogRepo) GetRecentByWorkspace(ctx context.Context, wsIDs []string, limit int) ([]entity.PaymentLog, error) {
	ctx, span := r.tracer.Start(ctx, "payment_log.repository.GetRecentByWorkspace")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	builder := database.PSQL.
		Select(
			"id::text", "workspace_id::text", "invoice_id", "event_type",
			"amount_paid", "COALESCE(payment_method,'') AS payment_method",
			"COALESCE(payment_channel,'') AS payment_channel",
			"COALESCE(payment_ref,'') AS payment_ref",
			"COALESCE(old_status,'') AS old_status", "COALESCE(new_status,'') AS new_status",
			"COALESCE(old_stage,'') AS old_stage", "COALESCE(new_stage,'') AS new_stage",
			"COALESCE(actor,'') AS actor", "COALESCE(notes,'') AS notes",
			"COALESCE(raw_payload, '{}'::jsonb) AS raw_payload", "timestamp",
		).
		From("payment_logs").
		Where(sq.Expr("workspace_id::text = ANY(?)", pq.Array(wsIDs))).
		OrderBy("timestamp DESC")

	if limit > 0 {
		builder = builder.Limit(uint64(limit))
	}

	query, args, err := builder.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	return r.scanLogs(ctx, query, args...)
}

func (r *paymentLogRepo) scanLogs(ctx context.Context, query string, args ...interface{}) ([]entity.PaymentLog, error) {
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query payment logs: %w", err)
	}
	defer rows.Close()

	var logs []entity.PaymentLog
	for rows.Next() {
		var pl entity.PaymentLog
		var rawJSON []byte
		if err := rows.Scan(
			&pl.ID, &pl.WorkspaceID, &pl.InvoiceID, &pl.EventType,
			&pl.AmountPaid, &pl.PaymentMethod, &pl.PaymentChannel, &pl.PaymentRef,
			&pl.OldStatus, &pl.NewStatus, &pl.OldStage, &pl.NewStage,
			&pl.Actor, &pl.Notes, &rawJSON, &pl.Timestamp,
		); err != nil {
			return nil, fmt.Errorf("scan payment log: %w", err)
		}
		if len(rawJSON) > 0 && string(rawJSON) != "{}" {
			if err := json.Unmarshal(rawJSON, &pl.RawPayload); err != nil {
				r.logger.Warn().Err(err).Str("invoice_id", pl.InvoiceID).Msg("failed to unmarshal raw_payload")
			}
		}
		logs = append(logs, pl)
	}
	return logs, rows.Err()
}
