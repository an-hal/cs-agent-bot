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
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// CustomFieldDefinitionRepository handles workspace-scoped custom field schemas.
type CustomFieldDefinitionRepository interface {
	List(ctx context.Context, workspaceID string) ([]entity.CustomFieldDefinition, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.CustomFieldDefinition, error)
	Create(ctx context.Context, workspaceID string, def *entity.CustomFieldDefinition) (*entity.CustomFieldDefinition, error)
	Update(ctx context.Context, workspaceID, id string, def *entity.CustomFieldDefinition) (*entity.CustomFieldDefinition, error)
	Delete(ctx context.Context, workspaceID, id string) error
	Reorder(ctx context.Context, workspaceID string, order []ReorderItem) error
}

// ReorderItem is one entry in the reorder payload.
type ReorderItem struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sort_order"`
}

type customFieldRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewCustomFieldDefinitionRepo constructs a CustomFieldDefinitionRepository.
func NewCustomFieldDefinitionRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) CustomFieldDefinitionRepository {
	return &customFieldRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *customFieldRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const cfdColumns = `id::text, workspace_id::text, field_key, field_label, field_type,
    is_required, COALESCE(default_value,''), COALESCE(placeholder,''), COALESCE(description,''),
    options, min_value, max_value, COALESCE(regex_pattern,''),
    sort_order, visible_in_table, column_width, created_at, updated_at`

func scanCFD(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.CustomFieldDefinition, error) {
	var (
		c          entity.CustomFieldDefinition
		optionsRaw []byte
		minVal     sql.NullFloat64
		maxVal     sql.NullFloat64
	)
	err := scanner.Scan(
		&c.ID, &c.WorkspaceID, &c.FieldKey, &c.FieldLabel, &c.FieldType,
		&c.IsRequired, &c.DefaultValue, &c.Placeholder, &c.Description,
		&optionsRaw, &minVal, &maxVal, &c.RegexPattern,
		&c.SortOrder, &c.VisibleInTable, &c.ColumnWidth, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(optionsRaw) > 0 {
		c.Options = json.RawMessage(optionsRaw)
	}
	if minVal.Valid {
		v := minVal.Float64
		c.MinValue = &v
	}
	if maxVal.Valid {
		v := maxVal.Float64
		c.MaxValue = &v
	}
	return &c, nil
}

func (r *customFieldRepo) List(ctx context.Context, workspaceID string) ([]entity.CustomFieldDefinition, error) {
	ctx, span := r.tracer.Start(ctx, "custom_field_definition.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(cfdColumns).From("custom_field_definitions").
		Where(sq.Expr("workspace_id::text = ?", workspaceID)).
		OrderBy("sort_order ASC", "field_key ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var out []entity.CustomFieldDefinition
	for rows.Next() {
		c, err := scanCFD(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (r *customFieldRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.CustomFieldDefinition, error) {
	ctx, span := r.tracer.Start(ctx, "custom_field_definition.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(cfdColumns).From("custom_field_definitions").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Expr("id::text = ?", id),
		}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	c, err := scanCFD(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query: %w", err)
	}
	return c, nil
}

func (r *customFieldRepo) Create(ctx context.Context, workspaceID string, def *entity.CustomFieldDefinition) (*entity.CustomFieldDefinition, error) {
	ctx, span := r.tracer.Start(ctx, "custom_field_definition.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var optionsArg interface{}
	if len(def.Options) > 0 {
		optionsArg = string(def.Options)
	}

	query, args, err := database.PSQL.
		Insert("custom_field_definitions").
		Columns("workspace_id", "field_key", "field_label", "field_type",
			"is_required", "default_value", "placeholder", "description", "options",
			"min_value", "max_value", "regex_pattern",
			"sort_order", "visible_in_table", "column_width").
		Values(workspaceID, def.FieldKey, def.FieldLabel, def.FieldType,
			def.IsRequired, nullIfEmpty(def.DefaultValue), nullIfEmpty(def.Placeholder), nullIfEmpty(def.Description), optionsArg,
			def.MinValue, def.MaxValue, nullIfEmpty(def.RegexPattern),
			def.SortOrder, def.VisibleInTable, def.ColumnWidth,
		).
		Suffix("RETURNING " + cfdColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	out, err := scanCFD(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("insert cfd: %w", err)
	}
	return out, nil
}

func (r *customFieldRepo) Update(ctx context.Context, workspaceID, id string, def *entity.CustomFieldDefinition) (*entity.CustomFieldDefinition, error) {
	ctx, span := r.tracer.Start(ctx, "custom_field_definition.repository.Update")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var optionsArg interface{}
	if len(def.Options) > 0 {
		optionsArg = string(def.Options)
	}

	upd := database.PSQL.Update("custom_field_definitions").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Expr("id::text = ?", id),
		}).
		Set("field_label", def.FieldLabel).
		Set("field_type", def.FieldType).
		Set("is_required", def.IsRequired).
		Set("default_value", nullIfEmpty(def.DefaultValue)).
		Set("placeholder", nullIfEmpty(def.Placeholder)).
		Set("description", nullIfEmpty(def.Description)).
		Set("options", optionsArg).
		Set("min_value", def.MinValue).
		Set("max_value", def.MaxValue).
		Set("regex_pattern", nullIfEmpty(def.RegexPattern)).
		Set("sort_order", def.SortOrder).
		Set("visible_in_table", def.VisibleInTable).
		Set("column_width", def.ColumnWidth)

	query, args, err := upd.Suffix("RETURNING " + cfdColumns).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update: %w", err)
	}
	out, err := scanCFD(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("update cfd: %w", err)
	}
	return out, nil
}

func (r *customFieldRepo) Delete(ctx context.Context, workspaceID, id string) error {
	ctx, span := r.tracer.Start(ctx, "custom_field_definition.repository.Delete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		"DELETE FROM custom_field_definitions WHERE workspace_id::text = $1 AND id::text = $2",
		workspaceID, id,
	)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("not found")
	}
	return nil
}

func (r *customFieldRepo) Reorder(ctx context.Context, workspaceID string, order []ReorderItem) error {
	ctx, span := r.tracer.Start(ctx, "custom_field_definition.repository.Reorder")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, item := range order {
		if _, err := tx.ExecContext(ctx,
			"UPDATE custom_field_definitions SET sort_order = $1 WHERE workspace_id::text = $2 AND id::text = $3",
			item.SortOrder, workspaceID, item.ID,
		); err != nil {
			return fmt.Errorf("reorder %s: %w", item.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
