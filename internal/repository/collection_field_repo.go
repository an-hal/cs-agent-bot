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

// CollectionFieldRepository accesses the `collection_fields` table.
// Per-collection (and therefore per-workspace) scoping is the caller's responsibility:
// the usecase MUST validate the collection_id against the workspace first.
type CollectionFieldRepository interface {
	ListByCollection(ctx context.Context, collectionID string) ([]entity.CollectionField, error)
	GetByID(ctx context.Context, collectionID, id string) (*entity.CollectionField, error)
	GetByKey(ctx context.Context, collectionID, key string) (*entity.CollectionField, error)
	Create(ctx context.Context, f *entity.CollectionField) (*entity.CollectionField, error)
	UpdateMeta(ctx context.Context, collectionID, id, label string, required bool, order int, options map[string]any) (*entity.CollectionField, error)
	Delete(ctx context.Context, collectionID, id string) error
	CountByCollection(ctx context.Context, collectionID string) (int, error)
	StripKeyFromRecords(ctx context.Context, collectionID, key string) error
}

type collectionFieldRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewCollectionFieldRepo constructs a CollectionFieldRepository.
func NewCollectionFieldRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) CollectionFieldRepository {
	return &collectionFieldRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *collectionFieldRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const fieldColumns = `id::text, collection_id::text, key, label, type, required,
    COALESCE(options, '{}'::jsonb), default_value, "order", created_at, updated_at`

func scanField(scanner interface {
	Scan(dest ...any) error
}) (*entity.CollectionField, error) {
	var f entity.CollectionField
	var optsRaw, defRaw []byte
	if err := scanner.Scan(
		&f.ID, &f.CollectionID, &f.Key, &f.Label, &f.Type, &f.Required,
		&optsRaw, &defRaw, &f.Order, &f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return nil, err
	}
	f.Options = map[string]any{}
	if len(optsRaw) > 0 {
		_ = json.Unmarshal(optsRaw, &f.Options)
	}
	if len(defRaw) > 0 {
		_ = json.Unmarshal(defRaw, &f.DefaultValue)
	}
	return &f, nil
}

func (r *collectionFieldRepo) ListByCollection(ctx context.Context, collectionID string) ([]entity.CollectionField, error) {
	ctx, span := r.tracer.Start(ctx, "collection_field.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `SELECT ` + fieldColumns + ` FROM collection_fields
		WHERE collection_id = $1 ORDER BY "order" ASC, created_at ASC`
	rows, err := r.db.QueryContext(ctx, q, collectionID)
	if err != nil {
		return nil, fmt.Errorf("collection_field list: %w", err)
	}
	defer rows.Close()

	out := []entity.CollectionField{}
	for rows.Next() {
		f, err := scanField(rows)
		if err != nil {
			return nil, fmt.Errorf("collection_field scan: %w", err)
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

func (r *collectionFieldRepo) GetByID(ctx context.Context, collectionID, id string) (*entity.CollectionField, error) {
	ctx, span := r.tracer.Start(ctx, "collection_field.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `SELECT ` + fieldColumns + ` FROM collection_fields
		WHERE id = $1 AND collection_id = $2`
	row := r.db.QueryRowContext(ctx, q, id, collectionID)
	f, err := scanField(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collection_field get: %w", err)
	}
	return f, nil
}

func (r *collectionFieldRepo) GetByKey(ctx context.Context, collectionID, key string) (*entity.CollectionField, error) {
	ctx, span := r.tracer.Start(ctx, "collection_field.repository.GetByKey")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `SELECT ` + fieldColumns + ` FROM collection_fields
		WHERE collection_id = $1 AND key = $2`
	row := r.db.QueryRowContext(ctx, q, collectionID, key)
	f, err := scanField(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collection_field get-by-key: %w", err)
	}
	return f, nil
}

func (r *collectionFieldRepo) Create(ctx context.Context, f *entity.CollectionField) (*entity.CollectionField, error) {
	ctx, span := r.tracer.Start(ctx, "collection_field.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	optsRaw, err := json.Marshal(coalesceMap(f.Options))
	if err != nil {
		return nil, fmt.Errorf("marshal options: %w", err)
	}
	var defRaw []byte
	if f.DefaultValue != nil {
		defRaw, err = json.Marshal(f.DefaultValue)
		if err != nil {
			return nil, fmt.Errorf("marshal default_value: %w", err)
		}
	}

	q := `INSERT INTO collection_fields
		(collection_id, key, label, type, required, options, default_value, "order")
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING ` + fieldColumns
	row := r.db.QueryRowContext(ctx, q,
		f.CollectionID, f.Key, f.Label, f.Type, f.Required, optsRaw, defRaw, f.Order)
	out, err := scanField(row)
	if err != nil {
		return nil, fmt.Errorf("collection_field insert: %w", err)
	}
	return out, nil
}

func (r *collectionFieldRepo) UpdateMeta(ctx context.Context, collectionID, id, label string, required bool, order int, options map[string]any) (*entity.CollectionField, error) {
	ctx, span := r.tracer.Start(ctx, "collection_field.repository.UpdateMeta")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	optsRaw, err := json.Marshal(coalesceMap(options))
	if err != nil {
		return nil, fmt.Errorf("marshal options: %w", err)
	}

	q := `UPDATE collection_fields
		SET label = $1, required = $2, "order" = $3, options = $4, updated_at = NOW()
		WHERE id = $5 AND collection_id = $6
		RETURNING ` + fieldColumns
	row := r.db.QueryRowContext(ctx, q, label, required, order, optsRaw, id, collectionID)
	out, err := scanField(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collection_field update: %w", err)
	}
	return out, nil
}

func (r *collectionFieldRepo) Delete(ctx context.Context, collectionID, id string) error {
	ctx, span := r.tracer.Start(ctx, "collection_field.repository.Delete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `DELETE FROM collection_fields WHERE id = $1 AND collection_id = $2`
	res, err := r.db.ExecContext(ctx, q, id, collectionID)
	if err != nil {
		return fmt.Errorf("collection_field delete: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *collectionFieldRepo) CountByCollection(ctx context.Context, collectionID string) (int, error) {
	ctx, span := r.tracer.Start(ctx, "collection_field.repository.CountByCollection")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var n int
	q := `SELECT COUNT(*) FROM collection_fields WHERE collection_id = $1`
	if err := r.db.QueryRowContext(ctx, q, collectionID).Scan(&n); err != nil {
		return 0, fmt.Errorf("collection_field count: %w", err)
	}
	return n, nil
}

// StripKeyFromRecords removes a key from every record's JSONB data when a field
// is deleted. Deletion is rare and records per collection are capped at 10k, so
// a single UPDATE is acceptable.
func (r *collectionFieldRepo) StripKeyFromRecords(ctx context.Context, collectionID, key string) error {
	ctx, span := r.tracer.Start(ctx, "collection_field.repository.StripKeyFromRecords")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	// `data - $2` removes the key. $2 is a bind parameter so no SQL-injection
	// surface exists even if the caller forgot to validate the key.
	q := `UPDATE collection_records SET data = data - $2::text, updated_at = NOW()
		WHERE collection_id = $1 AND data ? $2::text`
	if _, err := r.db.ExecContext(ctx, q, collectionID, key); err != nil {
		return fmt.Errorf("collection_field strip key: %w", err)
	}
	return nil
}
