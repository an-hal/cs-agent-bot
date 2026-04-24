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

type UserPreferencesRepository interface {
	Get(ctx context.Context, workspaceID, userEmail, namespace string) (*entity.UserPreference, error)
	List(ctx context.Context, workspaceID, userEmail string) ([]entity.UserPreference, error)
	Upsert(ctx context.Context, p *entity.UserPreference) (*entity.UserPreference, error)
	Delete(ctx context.Context, workspaceID, userEmail, namespace string) error
}

type userPreferencesRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewUserPreferencesRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) UserPreferencesRepository {
	return &userPreferencesRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *userPreferencesRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const userPrefColumns = "id::text, workspace_id::text, user_email, namespace, value, created_at, updated_at"

func scanUserPref(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.UserPreference, error) {
	var p entity.UserPreference
	var raw []byte
	err := scanner.Scan(&p.ID, &p.WorkspaceID, &p.UserEmail, &p.Namespace, &raw, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p.Value)
	}
	if p.Value == nil {
		p.Value = map[string]any{}
	}
	return &p, nil
}

func (r *userPreferencesRepo) Get(ctx context.Context, workspaceID, userEmail, namespace string) (*entity.UserPreference, error) {
	ctx, span := r.tracer.Start(ctx, "user_preferences.repository.Get")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(userPrefColumns).
		From("user_preferences").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Eq{"user_email": userEmail},
			sq.Eq{"namespace": namespace},
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build select: %w", err)
	}
	out, err := scanUserPref(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query user pref: %w", err)
	}
	return out, nil
}

func (r *userPreferencesRepo) List(ctx context.Context, workspaceID, userEmail string) ([]entity.UserPreference, error) {
	ctx, span := r.tracer.Start(ctx, "user_preferences.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(userPrefColumns).
		From("user_preferences").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Eq{"user_email": userEmail},
		}).
		OrderBy("namespace ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query list: %w", err)
	}
	defer rows.Close()

	var out []entity.UserPreference
	for rows.Next() {
		p, err := scanUserPref(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user pref: %w", err)
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *userPreferencesRepo) Upsert(ctx context.Context, p *entity.UserPreference) (*entity.UserPreference, error) {
	ctx, span := r.tracer.Start(ctx, "user_preferences.repository.Upsert")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	raw, err := json.Marshal(coalesceMap(p.Value))
	if err != nil {
		return nil, fmt.Errorf("marshal value: %w", err)
	}

	// ON CONFLICT updates value + updated_at, returns the final row.
	query := `
        INSERT INTO user_preferences (workspace_id, user_email, namespace, value)
        VALUES ($1, $2, $3, $4::jsonb)
        ON CONFLICT (workspace_id, user_email, namespace)
        DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
        RETURNING ` + userPrefColumns
	out, err := scanUserPref(r.db.QueryRowContext(ctx, query,
		p.WorkspaceID, p.UserEmail, p.Namespace, string(raw),
	))
	if err != nil {
		return nil, fmt.Errorf("upsert user pref: %w", err)
	}
	return out, nil
}

func (r *userPreferencesRepo) Delete(ctx context.Context, workspaceID, userEmail, namespace string) error {
	ctx, span := r.tracer.Start(ctx, "user_preferences.repository.Delete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		"DELETE FROM user_preferences WHERE workspace_id::text = $1 AND user_email = $2 AND namespace = $3",
		workspaceID, userEmail, namespace,
	)
	if err != nil {
		return fmt.Errorf("delete user pref: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("preference not found")
	}
	return nil
}
