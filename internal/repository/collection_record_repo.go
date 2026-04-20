package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// CollectionRecordRepository accesses the `collection_records` table.
// All field-key inputs arriving on ListOptions / DistinctOptions MUST have been
// validated against the collection's schema by the usecase — this repository
// trusts those strings when it interpolates them into SQL.
type CollectionRecordRepository interface {
	List(ctx context.Context, opts entity.CollectionRecordListOptions) ([]entity.CollectionRecord, int, error)
	Get(ctx context.Context, collectionID, id string) (*entity.CollectionRecord, error)
	CountActiveByCollection(ctx context.Context, collectionID string) (int, error)
	Create(ctx context.Context, rec *entity.CollectionRecord) (*entity.CollectionRecord, error)
	Update(ctx context.Context, collectionID, id string, data map[string]any) (*entity.CollectionRecord, error)
	SoftDelete(ctx context.Context, collectionID, id string) error
	BulkSoftDelete(ctx context.Context, collectionID string, ids []string) (int, error)
	BulkUpdate(ctx context.Context, collectionID string, ids []string, patch map[string]any) (int, error)
	Distinct(ctx context.Context, opts entity.DistinctOptions) ([]string, bool, error)
}

type collectionRecordRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewCollectionRecordRepo constructs a CollectionRecordRepository.
func NewCollectionRecordRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) CollectionRecordRepository {
	return &collectionRecordRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *collectionRecordRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const recordColumns = `id::text, collection_id::text, COALESCE(data, '{}'::jsonb),
    created_by, created_at, updated_at, deleted_at`

func scanRecord(scanner interface {
	Scan(dest ...any) error
}) (*entity.CollectionRecord, error) {
	var rec entity.CollectionRecord
	var dataRaw []byte
	if err := scanner.Scan(
		&rec.ID, &rec.CollectionID, &dataRaw,
		&rec.CreatedBy, &rec.CreatedAt, &rec.UpdatedAt, &rec.DeletedAt,
	); err != nil {
		return nil, err
	}
	rec.Data = map[string]any{}
	if len(dataRaw) > 0 {
		_ = json.Unmarshal(dataRaw, &rec.Data)
	}
	return &rec, nil
}

// orderByForSort returns a safe ORDER BY clause for the pre-validated sort key.
// SortKey is trusted by construction (usecase validates against schema).
func orderByForSort(sortKey, sortType string, desc bool) string {
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	switch sortKey {
	case "", "created_at":
		return "created_at " + dir
	case "updated_at":
		return "updated_at " + dir
	}
	// JSONB path with type-appropriate cast. SortKey was validated against
	// `^[a-z][a-z0-9_]{0,63}$` by the usecase, so direct interpolation is safe.
	switch sortType {
	case entity.ColFieldNumber:
		return fmt.Sprintf("NULLIF(data->>'%s','')::numeric NULLS LAST %s", sortKey, dir)
	case entity.ColFieldBoolean:
		return fmt.Sprintf("(data->>'%s')::boolean %s", sortKey, dir)
	case entity.ColFieldDate, entity.ColFieldDateTime:
		return fmt.Sprintf("NULLIF(data->>'%s','')::timestamptz NULLS LAST %s", sortKey, dir)
	default:
		return fmt.Sprintf("LOWER(data->>'%s') %s", sortKey, dir)
	}
}

func (r *collectionRecordRepo) List(ctx context.Context, opts entity.CollectionRecordListOptions) ([]entity.CollectionRecord, int, error) {
	ctx, span := r.tracer.Start(ctx, "collection_record.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	where := []string{"collection_id = $1", "deleted_at IS NULL"}
	args := []any{opts.CollectionID}

	if opts.FilterSQL != "" {
		where = append(where, opts.FilterSQL)
		args = append(args, opts.FilterArgs...)
	}
	if opts.Search != "" {
		// Full-text fallback via `data::text ILIKE`. GIN index isn't used but
		// dataset is capped at 10k so this is acceptable for MVP.
		args = append(args, "%"+opts.Search+"%")
		where = append(where, fmt.Sprintf("data::text ILIKE $%d", len(args)))
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}

	order := orderByForSort(opts.SortKey, opts.SortType, opts.SortDesc)
	q := `SELECT ` + recordColumns + ` FROM collection_records
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY ` + order + `
		LIMIT ` + fmt.Sprint(limit) + ` OFFSET ` + fmt.Sprint(opts.Offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("collection_record list: %w", err)
	}
	defer rows.Close()

	out := []entity.CollectionRecord{}
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("collection_record scan: %w", err)
		}
		out = append(out, *rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Count uses the same WHERE but no ORDER/LIMIT.
	countQ := `SELECT COUNT(*) FROM collection_records WHERE ` + strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("collection_record count: %w", err)
	}
	return out, total, nil
}

func (r *collectionRecordRepo) Get(ctx context.Context, collectionID, id string) (*entity.CollectionRecord, error) {
	ctx, span := r.tracer.Start(ctx, "collection_record.repository.Get")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `SELECT ` + recordColumns + ` FROM collection_records
		WHERE id = $1 AND collection_id = $2 AND deleted_at IS NULL`
	row := r.db.QueryRowContext(ctx, q, id, collectionID)
	rec, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collection_record get: %w", err)
	}
	return rec, nil
}

func (r *collectionRecordRepo) CountActiveByCollection(ctx context.Context, collectionID string) (int, error) {
	ctx, span := r.tracer.Start(ctx, "collection_record.repository.CountActiveByCollection")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var n int
	q := `SELECT COUNT(*) FROM collection_records
		WHERE collection_id = $1 AND deleted_at IS NULL`
	if err := r.db.QueryRowContext(ctx, q, collectionID).Scan(&n); err != nil {
		return 0, fmt.Errorf("collection_record count: %w", err)
	}
	return n, nil
}

func (r *collectionRecordRepo) Create(ctx context.Context, rec *entity.CollectionRecord) (*entity.CollectionRecord, error) {
	ctx, span := r.tracer.Start(ctx, "collection_record.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	dataRaw, err := json.Marshal(coalesceMap(rec.Data))
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}
	q := `INSERT INTO collection_records (collection_id, data, created_by)
		VALUES ($1,$2,$3)
		RETURNING ` + recordColumns
	row := r.db.QueryRowContext(ctx, q, rec.CollectionID, dataRaw, rec.CreatedBy)
	out, err := scanRecord(row)
	if err != nil {
		return nil, fmt.Errorf("collection_record insert: %w", err)
	}
	return out, nil
}

func (r *collectionRecordRepo) Update(ctx context.Context, collectionID, id string, data map[string]any) (*entity.CollectionRecord, error) {
	ctx, span := r.tracer.Start(ctx, "collection_record.repository.Update")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	dataRaw, err := json.Marshal(coalesceMap(data))
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}
	// Merge: `data || $new` preserves keys not present in the patch so PATCH
	// semantics hold even when caller sends a subset.
	q := `UPDATE collection_records
		SET data = COALESCE(data, '{}'::jsonb) || $1::jsonb, updated_at = NOW()
		WHERE id = $2 AND collection_id = $3 AND deleted_at IS NULL
		RETURNING ` + recordColumns
	row := r.db.QueryRowContext(ctx, q, dataRaw, id, collectionID)
	out, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("collection_record update: %w", err)
	}
	return out, nil
}

func (r *collectionRecordRepo) SoftDelete(ctx context.Context, collectionID, id string) error {
	ctx, span := r.tracer.Start(ctx, "collection_record.repository.SoftDelete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `UPDATE collection_records SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND collection_id = $2 AND deleted_at IS NULL`
	res, err := r.db.ExecContext(ctx, q, id, collectionID)
	if err != nil {
		return fmt.Errorf("collection_record delete: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *collectionRecordRepo) BulkSoftDelete(ctx context.Context, collectionID string, ids []string) (int, error) {
	ctx, span := r.tracer.Start(ctx, "collection_record.repository.BulkSoftDelete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if len(ids) == 0 {
		return 0, nil
	}
	q := `UPDATE collection_records SET deleted_at = NOW(), updated_at = NOW()
		WHERE collection_id = $1 AND deleted_at IS NULL AND id = ANY($2::uuid[])`
	res, err := r.db.ExecContext(ctx, q, collectionID, stringsToUUIDArray(ids))
	if err != nil {
		return 0, fmt.Errorf("collection_record bulk delete: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *collectionRecordRepo) BulkUpdate(ctx context.Context, collectionID string, ids []string, patch map[string]any) (int, error) {
	ctx, span := r.tracer.Start(ctx, "collection_record.repository.BulkUpdate")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if len(ids) == 0 || len(patch) == 0 {
		return 0, nil
	}
	patchRaw, err := json.Marshal(patch)
	if err != nil {
		return 0, fmt.Errorf("marshal patch: %w", err)
	}
	q := `UPDATE collection_records
		SET data = COALESCE(data, '{}'::jsonb) || $1::jsonb, updated_at = NOW()
		WHERE collection_id = $2 AND deleted_at IS NULL AND id = ANY($3::uuid[])`
	res, err := r.db.ExecContext(ctx, q, patchRaw, collectionID, stringsToUUIDArray(ids))
	if err != nil {
		return 0, fmt.Errorf("collection_record bulk update: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *collectionRecordRepo) Distinct(ctx context.Context, opts entity.DistinctOptions) ([]string, bool, error) {
	ctx, span := r.tracer.Start(ctx, "collection_record.repository.Distinct")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	limit := opts.Limit
	if limit <= 0 || limit > entity.MaxDistinctValues {
		limit = entity.MaxDistinctValues
	}

	where := []string{"collection_id = $1", "deleted_at IS NULL"}
	args := []any{opts.CollectionID}
	if opts.FilterSQL != "" {
		where = append(where, opts.FilterSQL)
		args = append(args, opts.FilterArgs...)
	}

	// Fetch one extra to detect truncation deterministically.
	fetch := limit + 1

	// Select expression per field type. field key is pre-validated by the usecase.
	var selectExpr string
	switch opts.FieldType {
	case entity.ColFieldMultiEnum:
		// Flatten array items into rows.
		selectExpr = fmt.Sprintf(`jsonb_array_elements_text(
			CASE WHEN jsonb_typeof(data->'%s') = 'array'
				THEN data->'%s' ELSE '[]'::jsonb END)`,
			opts.FieldKey, opts.FieldKey)
	case entity.ColFieldDate, entity.ColFieldDateTime:
		// Collapse to YYYY-MM-DD.
		selectExpr = fmt.Sprintf(`SUBSTRING(COALESCE(data->>'%s','') FROM 1 FOR 10)`, opts.FieldKey)
	default:
		selectExpr = fmt.Sprintf(`COALESCE(data->>'%s','')`, opts.FieldKey)
	}

	order := "v ASC"
	if opts.FieldType == entity.ColFieldNumber {
		order = "NULLIF(v,'')::numeric ASC NULLS LAST"
	}

	q := fmt.Sprintf(`
		SELECT DISTINCT v FROM (
			SELECT %s AS v FROM collection_records WHERE %s
		) sub
		WHERE v IS NOT NULL AND v <> ''
		ORDER BY %s
		LIMIT %d`,
		selectExpr, strings.Join(where, " AND "), order, fetch)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, false, fmt.Errorf("collection_record distinct: %w", err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var v sql.NullString
		if err := rows.Scan(&v); err != nil {
			return nil, false, fmt.Errorf("collection_record distinct scan: %w", err)
		}
		if v.Valid && v.String != "" {
			values = append(values, v.String)
		}
	}
	truncated := false
	if len(values) > limit {
		values = values[:limit]
		truncated = true
	}
	if values == nil {
		values = []string{}
	}
	return values, truncated, rows.Err()
}

// stringsToUUIDArray formats ids as a Postgres array literal `{uuid1,uuid2,...}`.
// Only call with UUID strings — no validation here.
func stringsToUUIDArray(ids []string) string {
	return "{" + strings.Join(ids, ",") + "}"
}
