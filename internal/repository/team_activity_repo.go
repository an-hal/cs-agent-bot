package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// TeamActivityLog is one row of the team-scoped audit stream.
type TeamActivityLog struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	ActorEmail  string         `json:"actor_email"`
	Action      string         `json:"action"`
	TargetEmail string         `json:"target_email,omitempty"`
	RoleID      string         `json:"role_id,omitempty"`
	Detail      map[string]any `json:"detail,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type TeamActivityLogRepository interface {
	Append(ctx context.Context, e *TeamActivityLog) error
	List(ctx context.Context, workspaceID string, limit int) ([]TeamActivityLog, error)
}

type teamActivityRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewTeamActivityLogRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) TeamActivityLogRepository {
	return &teamActivityRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *teamActivityRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *teamActivityRepo) Append(ctx context.Context, e *TeamActivityLog) error {
	ctx, span := r.tracer.Start(ctx, "team_activity.repository.Append")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	raw, _ := json.Marshal(coalesceMap(e.Detail))
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	var roleParam any
	if e.RoleID != "" {
		roleParam = e.RoleID
	}
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO team_activity_logs
            (workspace_id, actor_email, action, target_email, role_id, detail)
        VALUES ($1::uuid, $2, $3, $4, $5::uuid, $6::jsonb)`,
		e.WorkspaceID, e.ActorEmail, e.Action, e.TargetEmail, roleParam, string(raw),
	)
	if err != nil {
		return fmt.Errorf("append team activity: %w", err)
	}
	return nil
}

func (r *teamActivityRepo) List(ctx context.Context, workspaceID string, limit int) ([]TeamActivityLog, error) {
	ctx, span := r.tracer.Start(ctx, "team_activity.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
        SELECT id::text, workspace_id::text, actor_email, action,
               COALESCE(target_email,''), COALESCE(role_id::text,''),
               COALESCE(detail,'{}'::jsonb), created_at
          FROM team_activity_logs
         WHERE workspace_id::text = $1
         ORDER BY created_at DESC
         LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list team activity: %w", err)
	}
	defer rows.Close()
	var out []TeamActivityLog
	for rows.Next() {
		var e TeamActivityLog
		var raw []byte
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.ActorEmail, &e.Action,
			&e.TargetEmail, &e.RoleID, &raw, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &e.Detail)
		out = append(out, e)
	}
	return out, rows.Err()
}
