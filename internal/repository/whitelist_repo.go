package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// WhitelistRepository provides data access for the whitelist table.
type WhitelistRepository interface {
	IsAllowed(ctx context.Context, email string) (bool, error)
	List(ctx context.Context) ([]entity.WhitelistEntry, error)
	GetByEmail(ctx context.Context, email string) (*entity.WhitelistEntry, error)
	GetByID(ctx context.Context, id string) (*entity.WhitelistEntry, error)
	Create(ctx context.Context, email, addedBy, notes string) (*entity.WhitelistEntry, error)
	SetActive(ctx context.Context, id string, isActive bool) error
	Delete(ctx context.Context, id string) error
}

// ErrWhitelistDuplicate is returned by Create when an email already exists.
var ErrWhitelistDuplicate = errors.New("whitelist: email already exists")

// ErrWhitelistNotFound is returned when an entry can't be located.
var ErrWhitelistNotFound = errors.New("whitelist: entry not found")

type whitelistRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewWhitelistRepo constructs a postgres-backed WhitelistRepository.
func NewWhitelistRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) WhitelistRepository {
	return &whitelistRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *whitelistRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// IsAllowed reports whether the given email is on the whitelist and active.
func (r *whitelistRepo) IsAllowed(ctx context.Context, email string) (bool, error) {
	ctx, span := r.tracer.Start(ctx, "whitelist.repository.IsAllowed")
	defer span.End()

	email = normalizeEmail(email)
	if email == "" {
		return false, nil
	}

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("1").
		From("whitelist").
		Where(sq.Expr("LOWER(email) = ?", email)).
		Where(sq.Eq{"is_active": true}).
		Limit(1).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build IsAllowed query: %w", err)
	}

	var dummy int
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&dummy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("query IsAllowed: %w", err)
	}
	return true, nil
}

// List returns all whitelist entries ordered by creation time descending.
func (r *whitelistRepo) List(ctx context.Context) ([]entity.WhitelistEntry, error) {
	ctx, span := r.tracer.Start(ctx, "whitelist.repository.List")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("id", "email", "is_active", "added_by", "notes", "created_at", "updated_at").
		From("whitelist").
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build List query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query List whitelist: %w", err)
	}
	defer rows.Close()

	var out []entity.WhitelistEntry
	for rows.Next() {
		entry, scanErr := scanWhitelistRow(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate whitelist rows: %w", err)
	}
	return out, nil
}

// GetByEmail returns the whitelist entry with the given email (case-insensitive).
// Returns ErrWhitelistNotFound when no row matches.
func (r *whitelistRepo) GetByEmail(ctx context.Context, email string) (*entity.WhitelistEntry, error) {
	ctx, span := r.tracer.Start(ctx, "whitelist.repository.GetByEmail")
	defer span.End()

	email = normalizeEmail(email)
	if email == "" {
		return nil, ErrWhitelistNotFound
	}

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("id", "email", "is_active", "added_by", "notes", "created_at", "updated_at").
		From("whitelist").
		Where(sq.Expr("LOWER(email) = ?", email)).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build GetByEmail query: %w", err)
	}
	row := r.db.QueryRowContext(ctx, query, args...)
	entry, err := scanWhitelistRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWhitelistNotFound
		}
		return nil, err
	}
	return &entry, nil
}

// GetByID returns the whitelist entry with the given UUID.
func (r *whitelistRepo) GetByID(ctx context.Context, id string) (*entity.WhitelistEntry, error) {
	ctx, span := r.tracer.Start(ctx, "whitelist.repository.GetByID")
	defer span.End()

	if strings.TrimSpace(id) == "" {
		return nil, ErrWhitelistNotFound
	}

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("id", "email", "is_active", "added_by", "notes", "created_at", "updated_at").
		From("whitelist").
		Where(sq.Eq{"id": id}).
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build GetByID query: %w", err)
	}
	row := r.db.QueryRowContext(ctx, query, args...)
	entry, err := scanWhitelistRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrWhitelistNotFound
		}
		return nil, err
	}
	return &entry, nil
}

// Create inserts a new whitelist entry. Returns ErrWhitelistDuplicate if the
// email already exists.
func (r *whitelistRepo) Create(ctx context.Context, email, addedBy, notes string) (*entity.WhitelistEntry, error) {
	ctx, span := r.tracer.Start(ctx, "whitelist.repository.Create")
	defer span.End()

	email = normalizeEmail(email)
	if email == "" {
		return nil, fmt.Errorf("whitelist: empty email")
	}

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := `
		INSERT INTO whitelist (email, is_active, added_by, notes)
		VALUES ($1, TRUE, NULLIF($2, ''), NULLIF($3, ''))
		RETURNING id, email, is_active, added_by, notes, created_at, updated_at`

	row := r.db.QueryRowContext(ctx, query, email, addedBy, notes)
	entry, err := scanWhitelistRow(row)
	if err != nil {
		// Postgres unique violation surfaces in error string; keep this dependency
		// free of pq-specific imports by string-matching the well-known constraint.
		if strings.Contains(err.Error(), "uq_whitelist_email") || strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrWhitelistDuplicate
		}
		return nil, err
	}
	return &entry, nil
}

// SetActive toggles is_active for the given entry id.
func (r *whitelistRepo) SetActive(ctx context.Context, id string, isActive bool) error {
	ctx, span := r.tracer.Start(ctx, "whitelist.repository.SetActive")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := `UPDATE whitelist SET is_active = $1, updated_at = NOW() WHERE id = $2`
	res, err := r.db.ExecContext(ctx, query, isActive, id)
	if err != nil {
		return fmt.Errorf("update whitelist active: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrWhitelistNotFound
	}
	return nil
}

// Delete soft-deletes by setting is_active = false. We never hard delete in order
// to preserve audit history of who has had access.
func (r *whitelistRepo) Delete(ctx context.Context, id string) error {
	return r.SetActive(ctx, id, false)
}

// rowScanner is implemented by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanWhitelistRow(s rowScanner) (entity.WhitelistEntry, error) {
	var (
		entry     entity.WhitelistEntry
		addedBy   sql.NullString
		notes     sql.NullString
		createdAt sql.NullTime
		updatedAt sql.NullTime
	)
	if err := s.Scan(&entry.ID, &entry.Email, &entry.IsActive, &addedBy, &notes, &createdAt, &updatedAt); err != nil {
		return entry, err
	}
	if addedBy.Valid {
		entry.AddedBy = addedBy.String
	}
	if notes.Valid {
		entry.Notes = notes.String
	}
	if createdAt.Valid {
		entry.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		entry.UpdatedAt = updatedAt.Time
	}
	return entry, nil
}
