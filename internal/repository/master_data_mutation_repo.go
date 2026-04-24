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

// MasterDataMutationRepository records dashboard write history.
type MasterDataMutationRepository interface {
	Append(ctx context.Context, m *entity.MasterDataMutation) error
	List(ctx context.Context, workspaceID string, since *time.Time, limit int) ([]entity.MasterDataMutation, error)
}

type masterDataMutationRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewMasterDataMutationRepo constructs a MasterDataMutationRepository.
func NewMasterDataMutationRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) MasterDataMutationRepository {
	return &masterDataMutationRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *masterDataMutationRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *masterDataMutationRepo) Append(ctx context.Context, m *entity.MasterDataMutation) error {
	ctx, span := r.tracer.Start(ctx, "master_data_mutation.repository.Append")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	prevRaw, err := json.Marshal(coalesceMap(m.PreviousValues))
	if err != nil {
		return fmt.Errorf("marshal prev: %w", err)
	}
	newRaw, err := json.Marshal(coalesceMap(m.NewValues))
	if err != nil {
		return fmt.Errorf("marshal new: %w", err)
	}

	src := m.Source
	if src == "" {
		src = entity.MutationSourceDashboard
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO master_data_mutations
            (workspace_id, master_data_id, company_id, company_name, action, source, actor_email,
             changed_fields, previous_values, new_values, note)
         VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11)`,
		m.WorkspaceID, m.MasterDataID, m.CompanyID, m.CompanyName, m.Action, src, m.ActorEmail,
		pq.Array(m.ChangedFields), string(prevRaw), string(newRaw), m.Note,
	)
	if err != nil {
		return fmt.Errorf("insert mutation: %w", err)
	}
	return nil
}

func (r *masterDataMutationRepo) List(ctx context.Context, workspaceID string, since *time.Time, limit int) ([]entity.MasterDataMutation, error) {
	ctx, span := r.tracer.Start(ctx, "master_data_mutation.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 500 {
		limit = 50
	}
	conds := sq.And{sq.Expr("workspace_id::text = ?", workspaceID)}
	if since != nil {
		conds = append(conds, sq.GtOrEq{"timestamp": *since})
	}
	query, args, err := database.PSQL.
		Select(`id::text, workspace_id::text, master_data_id::text, COALESCE(company_id,''),
            COALESCE(company_name,''), action, COALESCE(source,'dashboard'), actor_email, changed_fields,
            COALESCE(previous_values,'{}'::jsonb), COALESCE(new_values,'{}'::jsonb),
            COALESCE(note,''), timestamp`).
		From("master_data_mutations").
		Where(conds).
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
	var out []entity.MasterDataMutation
	for rows.Next() {
		var m entity.MasterDataMutation
		var prevRaw, newRaw []byte
		if err := rows.Scan(
			&m.ID, &m.WorkspaceID, &m.MasterDataID, &m.CompanyID, &m.CompanyName,
			&m.Action, &m.Source, &m.ActorEmail, pq.Array(&m.ChangedFields),
			&prevRaw, &newRaw, &m.Note, &m.Timestamp,
		); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		_ = json.Unmarshal(prevRaw, &m.PreviousValues)
		_ = json.Unmarshal(newRaw, &m.NewValues)
		if m.ChangedFields == nil {
			m.ChangedFields = []string{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func coalesceMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
