package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// WorkspaceTheme is the minimal shape returned by GetTheme / UpsertTheme.
type WorkspaceTheme struct {
	WorkspaceID string         `json:"workspace_id"`
	Theme       map[string]any `json:"theme"`
	UpdatedAt   time.Time      `json:"updated_at"`
	UpdatedBy   string         `json:"updated_by"`
}

type WorkspaceThemeRepository interface {
	Get(ctx context.Context, workspaceID string) (*WorkspaceTheme, error)
	Upsert(ctx context.Context, t *WorkspaceTheme) (*WorkspaceTheme, error)
	ExpandHolding(ctx context.Context, workspaceID string) ([]string, error)
}

type workspaceThemeRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewWorkspaceThemeRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) WorkspaceThemeRepository {
	return &workspaceThemeRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *workspaceThemeRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *workspaceThemeRepo) Get(ctx context.Context, workspaceID string) (*WorkspaceTheme, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_theme.repository.Get")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var t WorkspaceTheme
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
        SELECT workspace_id::text, theme, updated_at, updated_by
          FROM workspace_themes
         WHERE workspace_id::text = $1`,
		workspaceID,
	).Scan(&t.WorkspaceID, &raw, &t.UpdatedAt, &t.UpdatedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return empty default so FE always has something to render.
			return &WorkspaceTheme{WorkspaceID: workspaceID, Theme: map[string]any{}}, nil
		}
		return nil, fmt.Errorf("get theme: %w", err)
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &t.Theme)
	}
	if t.Theme == nil {
		t.Theme = map[string]any{}
	}
	return &t, nil
}

func (r *workspaceThemeRepo) Upsert(ctx context.Context, t *WorkspaceTheme) (*WorkspaceTheme, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_theme.repository.Upsert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	raw, err := json.Marshal(coalesceMap(t.Theme))
	if err != nil {
		return nil, fmt.Errorf("marshal theme: %w", err)
	}
	var out WorkspaceTheme
	var rawOut []byte
	err = r.db.QueryRowContext(ctx, `
        INSERT INTO workspace_themes (workspace_id, theme, updated_by)
        VALUES ($1::uuid, $2::jsonb, $3)
        ON CONFLICT (workspace_id) DO UPDATE SET
            theme = EXCLUDED.theme,
            updated_at = NOW(),
            updated_by = EXCLUDED.updated_by
        RETURNING workspace_id::text, theme, updated_at, updated_by`,
		t.WorkspaceID, string(raw), t.UpdatedBy,
	).Scan(&out.WorkspaceID, &rawOut, &out.UpdatedAt, &out.UpdatedBy)
	if err != nil {
		return nil, fmt.Errorf("upsert theme: %w", err)
	}
	if len(rawOut) > 0 {
		_ = json.Unmarshal(rawOut, &out.Theme)
	}
	if out.Theme == nil {
		out.Theme = map[string]any{}
	}
	return &out, nil
}

// ExpandHolding returns [workspaceID + sibling IDs] when the given workspace
// is the holding parent OR a child; otherwise [workspaceID]. Multi-workspace
// dashboards call this to aggregate across a holding without the caller
// having to know the hierarchy shape.
func (r *workspaceThemeRepo) ExpandHolding(ctx context.Context, workspaceID string) ([]string, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_theme.repository.ExpandHolding")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	// Determine holding_id: row's own holding_id, OR row's own id if it is a parent.
	var holdingID sql.NullString
	err := r.db.QueryRowContext(ctx, `
        SELECT COALESCE(holding_id::text, id::text)
          FROM workspaces
         WHERE id::text = $1`,
		workspaceID,
	).Scan(&holdingID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []string{workspaceID}, nil
		}
		return nil, fmt.Errorf("resolve holding: %w", err)
	}
	if !holdingID.Valid || holdingID.String == "" {
		return []string{workspaceID}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
        SELECT id::text FROM workspaces
         WHERE holding_id::text = $1 OR id::text = $1
         ORDER BY id`,
		holdingID.String,
	)
	if err != nil {
		return nil, fmt.Errorf("expand: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if len(out) == 0 {
		out = []string{workspaceID}
	}
	return out, rows.Err()
}
