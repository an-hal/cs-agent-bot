package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/secretvault"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type WorkspaceIntegrationRepository interface {
	GetByProvider(ctx context.Context, workspaceID, provider string) (*entity.WorkspaceIntegration, error)
	List(ctx context.Context, workspaceID string) ([]entity.WorkspaceIntegration, error)
	Upsert(ctx context.Context, w *entity.WorkspaceIntegration) (*entity.WorkspaceIntegration, error)
	Delete(ctx context.Context, workspaceID, provider string) error
}

type workspaceIntegrationRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
	vault        *secretvault.Vault // optional — nil means plaintext storage
}

func NewWorkspaceIntegrationRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) WorkspaceIntegrationRepository {
	return &workspaceIntegrationRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

// NewWorkspaceIntegrationRepoWithVault is like NewWorkspaceIntegrationRepo
// but encrypts secret-ish config keys on write and decrypts on read. Wire
// this when CONFIG_ENCRYPTION_KEY is set.
func NewWorkspaceIntegrationRepoWithVault(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger, vault *secretvault.Vault) WorkspaceIntegrationRepository {
	return &workspaceIntegrationRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger, vault: vault}
}

func (r *workspaceIntegrationRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const wsIntColumns = `id::text, workspace_id::text, provider, display_name, config,
    is_active, created_at, updated_at, created_by, updated_by`

func scanWsIntegration(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.WorkspaceIntegration, error) {
	var w entity.WorkspaceIntegration
	var raw []byte
	err := scanner.Scan(
		&w.ID, &w.WorkspaceID, &w.Provider, &w.DisplayName, &raw,
		&w.IsActive, &w.CreatedAt, &w.UpdatedAt, &w.CreatedBy, &w.UpdatedBy,
	)
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &w.Config)
	}
	if w.Config == nil {
		w.Config = map[string]any{}
	}
	return &w, nil
}

func (r *workspaceIntegrationRepo) GetByProvider(ctx context.Context, workspaceID, provider string) (*entity.WorkspaceIntegration, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_integration.repository.GetByProvider")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(wsIntColumns).
		From("workspace_integrations").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Eq{"provider": provider},
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select: %w", err)
	}
	out, err := scanWsIntegration(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query integration: %w", err)
	}
	_ = r.vault.DecryptMap(out.Config)
	return out, nil
}

func (r *workspaceIntegrationRepo) List(ctx context.Context, workspaceID string) ([]entity.WorkspaceIntegration, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_integration.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(wsIntColumns).
		From("workspace_integrations").
		Where(sq.Expr("workspace_id::text = ?", workspaceID)).
		OrderBy("provider ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query integrations: %w", err)
	}
	defer rows.Close()

	var out []entity.WorkspaceIntegration
	for rows.Next() {
		w, err := scanWsIntegration(rows)
		if err != nil {
			return nil, fmt.Errorf("scan integration: %w", err)
		}
		_ = r.vault.DecryptMap(w.Config)
		out = append(out, *w)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *workspaceIntegrationRepo) Upsert(ctx context.Context, w *entity.WorkspaceIntegration) (*entity.WorkspaceIntegration, error) {
	ctx, span := r.tracer.Start(ctx, "workspace_integration.repository.Upsert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	// Encrypt sensitive config keys before persisting. Legacy (plaintext)
	// reads still work — Decrypt handles non-envelope values as passthrough.
	// Work on a shallow copy so the caller's map isn't mutated (FE may reuse it).
	toStore := map[string]any{}
	for k, v := range coalesceMap(w.Config) {
		toStore[k] = v
	}
	if err := r.vault.EncryptMap(toStore); err != nil {
		return nil, fmt.Errorf("encrypt config: %w", err)
	}
	raw, err := json.Marshal(toStore)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}

	query := `
        INSERT INTO workspace_integrations
            (workspace_id, provider, display_name, config, is_active, created_by, updated_by)
        VALUES ($1, $2, $3, $4::jsonb, $5, $6, $6)
        ON CONFLICT (workspace_id, provider) DO UPDATE SET
            display_name = EXCLUDED.display_name,
            config       = EXCLUDED.config,
            is_active    = EXCLUDED.is_active,
            updated_at   = NOW(),
            updated_by   = EXCLUDED.updated_by
        RETURNING ` + wsIntColumns
	out, err := scanWsIntegration(r.db.QueryRowContext(ctx, query,
		w.WorkspaceID, w.Provider, w.DisplayName, string(raw), w.IsActive, w.UpdatedBy,
	))
	if err != nil {
		return nil, fmt.Errorf("upsert integration: %w", err)
	}
	// Return decrypted view to caller — the DB stores encrypted, but callers
	// expect the same shape they sent in.
	_ = r.vault.DecryptMap(out.Config)
	return out, nil
}

func (r *workspaceIntegrationRepo) Delete(ctx context.Context, workspaceID, provider string) error {
	ctx, span := r.tracer.Start(ctx, "workspace_integration.repository.Delete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		"DELETE FROM workspace_integrations WHERE workspace_id::text = $1 AND provider = $2",
		workspaceID, provider,
	)
	if err != nil {
		return fmt.Errorf("delete integration: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("integration not found")
	}
	return nil
}
