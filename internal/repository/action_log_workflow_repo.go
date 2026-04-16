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
	"github.com/rs/zerolog"
)

// ActionLogWorkflowRepository writes the NEW plural action_logs table for
// workflow node execution traces. This is intentionally distinct from the
// existing singular `action_log` audit table.
type ActionLogWorkflowRepository interface {
	Append(ctx context.Context, l *entity.ActionLogWorkflow) error
	List(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ActionLogWorkflow, error)
}

type actionLogWorkflowRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewActionLogWorkflowRepo constructs an ActionLogWorkflowRepository.
func NewActionLogWorkflowRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ActionLogWorkflowRepository {
	return &actionLogWorkflowRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *actionLogWorkflowRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *actionLogWorkflowRepo) Append(ctx context.Context, l *entity.ActionLogWorkflow) error {
	ctx, span := r.tracer.Start(ctx, "action_log_workflow.repository.Append")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if l.FieldsRead == nil {
		l.FieldsRead = json.RawMessage(`{}`)
	}
	if l.FieldsWritten == nil {
		l.FieldsWritten = json.RawMessage(`{}`)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO action_logs (workspace_id, master_data_id, trigger_id, template_id,
            status, channel, phase, fields_read, fields_written, replied, conversation_id)
         VALUES ($1::uuid, $2::uuid, $3, NULLIF($4,''), $5, NULLIF($6,''), NULLIF($7,''),
            $8::jsonb, $9::jsonb, $10, NULLIF($11,''))`,
		l.WorkspaceID, l.MasterDataID, l.TriggerID, l.TemplateID,
		l.Status, l.Channel, l.Phase, string(l.FieldsRead), string(l.FieldsWritten),
		l.Replied, l.ConversationID,
	)
	if err != nil {
		return fmt.Errorf("insert action_log: %w", err)
	}
	return nil
}

func (r *actionLogWorkflowRepo) List(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ActionLogWorkflow, error) {
	ctx, span := r.tracer.Start(ctx, "action_log_workflow.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 500 {
		limit = 100
	}

	conds := sq.And{sq.Expr("workspace_id::text = ?", workspaceID)}
	if masterDataID != "" {
		conds = append(conds, sq.Expr("master_data_id::text = ?", masterDataID))
	}
	query, args, err := database.PSQL.
		Select(`id::text, workspace_id::text, master_data_id::text, trigger_id,
            COALESCE(template_id,''), status, COALESCE(channel,''), COALESCE(phase,''),
            COALESCE(fields_read,'{}'::jsonb), COALESCE(fields_written,'{}'::jsonb),
            replied, COALESCE(conversation_id,''), timestamp`).
		From("action_logs").Where(conds).
		OrderBy("timestamp DESC").
		Limit(uint64(limit)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var out []entity.ActionLogWorkflow
	for rows.Next() {
		var l entity.ActionLogWorkflow
		var read, written []byte
		if err := rows.Scan(
			&l.ID, &l.WorkspaceID, &l.MasterDataID, &l.TriggerID, &l.TemplateID,
			&l.Status, &l.Channel, &l.Phase, &read, &written,
			&l.Replied, &l.ConversationID, &l.Timestamp,
		); err != nil {
			return nil, err
		}
		l.FieldsRead = json.RawMessage(read)
		l.FieldsWritten = json.RawMessage(written)
		out = append(out, l)
	}
	return out, rows.Err()
}

// Stub to satisfy unused import linter if any of these imports ever drift.
var _ = sql.ErrNoRows
var _ = time.Now
