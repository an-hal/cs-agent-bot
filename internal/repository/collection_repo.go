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

// CollectionRepository is the data access layer for the `collections` meta table.
// Per-workspace scoping is enforced on every query.
type CollectionRepository interface {
	List(ctx context.Context, workspaceID string) ([]entity.Collection, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.Collection, error)
	GetBySlug(ctx context.Context, workspaceID, slug string) (*entity.Collection, error)
	Create(ctx context.Context, c *entity.Collection) (*entity.Collection, error)
	UpdateMeta(ctx context.Context, workspaceID, id string, name, description, icon string, permissions map[string]any) (*entity.Collection, error)
	SoftDelete(ctx context.Context, workspaceID, id string) error
	CountActiveByWorkspace(ctx context.Context, workspaceID string) (int, error)
}

type collectionRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewCollectionRepo constructs a CollectionRepository backed by PostgreSQL.
func NewCollectionRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) CollectionRepository {
	return &collectionRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *collectionRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const collectionColumns = `id::text, workspace_id::text, slug, name, description, icon,
    COALESCE(permissions, '{}'::jsonb), created_by, created_at, updated_at, deleted_at`

func scanCollection(scanner interface {
	Scan(dest ...any) error
}) (*entity.Collection, error) {
	var c entity.Collection
	var permsRaw []byte
	if err := scanner.Scan(
		&c.ID, &c.WorkspaceID, &c.Slug, &c.Name, &c.Description, &c.Icon,
		&permsRaw, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
	); err != nil {
		return nil, err
	}
	c.Permissions = map[string]any{}
	if len(permsRaw) > 0 {
		_ = json.Unmarshal(permsRaw, &c.Permissions)
	}
	return &c, nil
}

func (r *collectionRepo) List(ctx context.Context, workspaceID string) ([]entity.Collection, error) {
	ctx, span := r.tracer.Start(ctx, "collection.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `SELECT ` + collectionColumns + `,
		(SELECT COUNT(*) FROM collection_fields f WHERE f.collection_id = c.id) AS field_count,
		(SELECT COUNT(*) FROM collection_records r WHERE r.collection_id = c.id AND r.deleted_at IS NULL) AS record_count
		FROM collections c
		WHERE c.workspace_id = $1 AND c.deleted_at IS NULL
		ORDER BY c.name ASC`

	rows, err := r.db.QueryContext(ctx, q, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("collection list: %w", err)
	}
	defer rows.Close()

	out := []entity.Collection{}
	for rows.Next() {
		var c entity.Collection
		var permsRaw []byte
		if err := rows.Scan(
			&c.ID, &c.WorkspaceID, &c.Slug, &c.Name, &c.Description, &c.Icon,
			&permsRaw, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt,
			&c.FieldCount, &c.RecordCount,
		); err != nil {
			return nil, fmt.Errorf("collection scan: %w", err)
		}
		c.Permissions = map[string]any{}
		if len(permsRaw) > 0 {
			_ = json.Unmarshal(permsRaw, &c.Permissions)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *collectionRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.Collection, error) {
	ctx, span := r.tracer.Start(ctx, "collection.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `SELECT ` + collectionColumns + `
		FROM collections c
		WHERE c.id = $1 AND c.workspace_id = $2 AND c.deleted_at IS NULL`

	row := r.db.QueryRowContext(ctx, q, id, workspaceID)
	c, err := scanCollection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collection get: %w", err)
	}
	return c, nil
}

func (r *collectionRepo) GetBySlug(ctx context.Context, workspaceID, slug string) (*entity.Collection, error) {
	ctx, span := r.tracer.Start(ctx, "collection.repository.GetBySlug")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `SELECT ` + collectionColumns + `
		FROM collections c
		WHERE c.slug = $1 AND c.workspace_id = $2 AND c.deleted_at IS NULL`
	row := r.db.QueryRowContext(ctx, q, slug, workspaceID)
	c, err := scanCollection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collection get-by-slug: %w", err)
	}
	return c, nil
}

func (r *collectionRepo) Create(ctx context.Context, c *entity.Collection) (*entity.Collection, error) {
	ctx, span := r.tracer.Start(ctx, "collection.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	permsRaw, err := json.Marshal(coalesceMap(c.Permissions))
	if err != nil {
		return nil, fmt.Errorf("marshal permissions: %w", err)
	}

	q := `INSERT INTO collections (workspace_id, slug, name, description, icon, permissions, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING ` + collectionColumns
	row := r.db.QueryRowContext(ctx, q,
		c.WorkspaceID, c.Slug, c.Name, c.Description, c.Icon, permsRaw, c.CreatedBy)
	out, err := scanCollection(row)
	if err != nil {
		return nil, fmt.Errorf("collection insert: %w", err)
	}
	return out, nil
}

func (r *collectionRepo) UpdateMeta(ctx context.Context, workspaceID, id, name, description, icon string, permissions map[string]any) (*entity.Collection, error) {
	ctx, span := r.tracer.Start(ctx, "collection.repository.UpdateMeta")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	permsRaw, err := json.Marshal(coalesceMap(permissions))
	if err != nil {
		return nil, fmt.Errorf("marshal permissions: %w", err)
	}
	q := `UPDATE collections
		SET name = $1, description = $2, icon = $3, permissions = $4, updated_at = NOW()
		WHERE id = $5 AND workspace_id = $6 AND deleted_at IS NULL
		RETURNING ` + collectionColumns
	row := r.db.QueryRowContext(ctx, q, name, description, icon, permsRaw, id, workspaceID)
	out, err := scanCollection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collection update: %w", err)
	}
	return out, nil
}

func (r *collectionRepo) SoftDelete(ctx context.Context, workspaceID, id string) error {
	ctx, span := r.tracer.Start(ctx, "collection.repository.SoftDelete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `UPDATE collections SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, q, id, workspaceID)
	if err != nil {
		return fmt.Errorf("collection soft-delete: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *collectionRepo) CountActiveByWorkspace(ctx context.Context, workspaceID string) (int, error) {
	ctx, span := r.tracer.Start(ctx, "collection.repository.CountActiveByWorkspace")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var n int
	q := `SELECT COUNT(*) FROM collections WHERE workspace_id = $1 AND deleted_at IS NULL`
	if err := r.db.QueryRowContext(ctx, q, workspaceID).Scan(&n); err != nil {
		return 0, fmt.Errorf("collection count: %w", err)
	}
	return n, nil
}
