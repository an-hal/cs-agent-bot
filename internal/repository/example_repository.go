package repository

import (
	"context"
	"database/sql"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/google/uuid"
)

type ExampleRepository interface {
	FetchAll(ctx context.Context, params pagination.Params) ([]entity.Example, error)
	Count(ctx context.Context) (int64, error)
	FetchByID(ctx context.Context, id uuid.UUID) (*entity.Example, error)
	Store(ctx context.Context, tx *sql.Tx, example *entity.Example) error
	Remove(ctx context.Context, id uuid.UUID) error
}

type exampleRepo struct {
	DB           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
}

func NewExampleRepo(db *sql.DB, queryTimeout time.Duration, tracer tracer.Tracer) ExampleRepository {
	return &exampleRepo{
		DB:           db,
		queryTimeout: queryTimeout,
		tracer:       tracer,
	}
}

func (r *exampleRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *exampleRepo) FetchAll(ctx context.Context, params pagination.Params) ([]entity.Example, error) {
	ctx, span := r.tracer.Start(ctx, "example.repository.FetchAll")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("id", "name", "description", "status", "created_at", "updated_at").
		From("examples").
		Where(sq.Eq{"deleted_at": nil}).
		OrderBy("created_at DESC").
		Limit(uint64(params.Limit)).
		Offset(uint64(params.Offset)).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var examples []entity.Example
	for rows.Next() {
		var e entity.Example
		if err := rows.Scan(&e.ID, &e.Name, &e.Description, &e.Status, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		examples = append(examples, e)
	}

	return examples, nil
}

func (r *exampleRepo) Count(ctx context.Context) (int64, error) {
	ctx, span := r.tracer.Start(ctx, "example.repository.Count")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("COUNT(*)").
		From("examples").
		Where(sq.Eq{"deleted_at": nil}).
		ToSql()
	if err != nil {
		return 0, err
	}

	var count int64
	err = r.DB.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *exampleRepo) FetchByID(ctx context.Context, id uuid.UUID) (*entity.Example, error) {
	ctx, span := r.tracer.Start(ctx, "example.repository.FetchByID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("id", "name", "description", "status", "created_at", "updated_at").
		From("examples").
		Where(sq.And{
			sq.Eq{"id": id},
			sq.Eq{"deleted_at": nil},
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	var e entity.Example
	err = r.DB.QueryRowContext(ctx, query, args...).
		Scan(&e.ID, &e.Name, &e.Description, &e.Status, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &e, nil
}

func (r *exampleRepo) Store(ctx context.Context, tx *sql.Tx, example *entity.Example) error {
	ctx, span := r.tracer.Start(ctx, "example.repository.Store")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Insert("examples").
		Columns("name", "description", "status").
		Values(example.Name, example.Description, example.Status).
		Suffix("RETURNING id, created_at, updated_at").
		ToSql()
	if err != nil {
		return err
	}

	return tx.QueryRowContext(ctx, query, args...).
		Scan(&example.ID, &example.CreatedAt, &example.UpdatedAt)
}

func (r *exampleRepo) Remove(ctx context.Context, id uuid.UUID) error {
	ctx, span := r.tracer.Start(ctx, "example.repository.Remove")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Update("examples").
		Set("deleted_at", sq.Expr("NOW()")).
		Where(sq.And{
			sq.Eq{"id": id},
			sq.Eq{"deleted_at": nil},
		}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = r.DB.ExecContext(ctx, query, args...)
	return err
}
