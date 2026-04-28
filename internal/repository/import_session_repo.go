package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// ImportSessionRepository persists Phase C wizard state.
type ImportSessionRepository interface {
	Create(ctx context.Context, s *entity.ImportSession) (*entity.ImportSession, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.ImportSession, error)
	UpdateOverrides(ctx context.Context, workspaceID, id string, overrides map[string]map[string]string) error
	MarkSubmitted(ctx context.Context, workspaceID, id, approvalID string) error
}

type importSessionRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewImportSessionRepo constructs the import session repository.
func NewImportSessionRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ImportSessionRepository {
	return &importSessionRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *importSessionRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *importSessionRepo) Create(ctx context.Context, s *entity.ImportSession) (*entity.ImportSession, error) {
	ctx, span := r.tracer.Start(ctx, "import_session.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if s.WorkspaceID == "" {
		return nil, errors.New("workspace_id required")
	}
	if s.Status == "" {
		s.Status = entity.ImportSessionStatusPending
	}
	if s.Mode == "" {
		s.Mode = "add_new"
	}
	if s.Mapping == nil {
		s.Mapping = map[string]string{}
	}
	if s.CellOverrides == nil {
		s.CellOverrides = map[string]map[string]string{}
	}
	mappingJSON, err := json.Marshal(s.Mapping)
	if err != nil {
		return nil, fmt.Errorf("marshal mapping: %w", err)
	}
	overridesJSON, err := json.Marshal(s.CellOverrides)
	if err != nil {
		return nil, fmt.Errorf("marshal overrides: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
INSERT INTO import_sessions (
    workspace_id, created_by, status,
    file_name, file_b64, sheet_name, mode,
    mapping, cell_overrides
) VALUES (
    $1::uuid, $2, $3, $4, $5, NULLIF($6,''), $7, $8::jsonb, $9::jsonb
)
RETURNING id::text, created_at, updated_at, expires_at`,
		s.WorkspaceID, s.CreatedBy, s.Status,
		s.FileName, s.FileB64, s.SheetName, s.Mode,
		string(mappingJSON), string(overridesJSON),
	)
	if err := row.Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt, &s.ExpiresAt); err != nil {
		return nil, fmt.Errorf("insert import_session: %w", err)
	}
	return s, nil
}

func (r *importSessionRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.ImportSession, error) {
	ctx, span := r.tracer.Start(ctx, "import_session.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var (
		s             entity.ImportSession
		sheetName     sql.NullString
		approvalID    sql.NullString
		mappingRaw    []byte
		overridesRaw  []byte
	)
	err := r.db.QueryRowContext(ctx, `
SELECT id::text, workspace_id::text, created_by, status,
       file_name, file_b64, sheet_name, mode,
       mapping, cell_overrides,
       approval_id::text, created_at, updated_at, expires_at
FROM import_sessions
WHERE workspace_id::text = $1 AND id::text = $2`, workspaceID, id).Scan(
		&s.ID, &s.WorkspaceID, &s.CreatedBy, &s.Status,
		&s.FileName, &s.FileB64, &sheetName, &s.Mode,
		&mappingRaw, &overridesRaw,
		&approvalID, &s.CreatedAt, &s.UpdatedAt, &s.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get import_session: %w", err)
	}
	s.SheetName = sheetName.String
	if approvalID.Valid {
		s.ApprovalID = approvalID.String
	}
	if len(mappingRaw) > 0 {
		if err := json.Unmarshal(mappingRaw, &s.Mapping); err != nil {
			return nil, fmt.Errorf("unmarshal mapping: %w", err)
		}
	}
	if s.Mapping == nil {
		s.Mapping = map[string]string{}
	}
	if len(overridesRaw) > 0 {
		if err := json.Unmarshal(overridesRaw, &s.CellOverrides); err != nil {
			return nil, fmt.Errorf("unmarshal overrides: %w", err)
		}
	}
	if s.CellOverrides == nil {
		s.CellOverrides = map[string]map[string]string{}
	}
	return &s, nil
}

func (r *importSessionRepo) UpdateOverrides(ctx context.Context, workspaceID, id string, overrides map[string]map[string]string) error {
	ctx, span := r.tracer.Start(ctx, "import_session.repository.UpdateOverrides")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	overridesJSON, err := json.Marshal(overrides)
	if err != nil {
		return fmt.Errorf("marshal overrides: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `
UPDATE import_sessions
SET cell_overrides = $1::jsonb
WHERE workspace_id::text = $2 AND id::text = $3 AND status = 'pending'`,
		string(overridesJSON), workspaceID, id,
	)
	if err != nil {
		return fmt.Errorf("update overrides: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("session not found or not pending")
	}
	return nil
}

func (r *importSessionRepo) MarkSubmitted(ctx context.Context, workspaceID, id, approvalID string) error {
	ctx, span := r.tracer.Start(ctx, "import_session.repository.MarkSubmitted")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `
UPDATE import_sessions
SET status = 'submitted', approval_id = $1::uuid
WHERE workspace_id::text = $2 AND id::text = $3 AND status = 'pending'`,
		approvalID, workspaceID, id,
	)
	if err != nil {
		return fmt.Errorf("mark submitted: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("session not found or not pending")
	}
	return nil
}
