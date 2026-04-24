package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// RevokedSession is a single JTI-level revocation. The `ExpiresAt` field
// matches the underlying JWT's `exp` claim; housekeeping cron cleans expired
// rows so the lookup stays fast.
type RevokedSession struct {
	ID          string
	WorkspaceID string
	JTI         string
	UserEmail   string
	Reason      string
	RevokedBy   string
	RevokedAt   time.Time
	ExpiresAt   time.Time
}

type RevokedSessionRepository interface {
	// Revoke inserts a revocation; caller supplies expires_at from the JWT.
	Revoke(ctx context.Context, r *RevokedSession) error
	// IsRevoked returns true when the given JTI is in the revocation list and
	// has not yet expired. Hot path — called on every JWT-auth request.
	IsRevoked(ctx context.Context, jti string) (bool, error)
	// ListByUser returns active revocations for a user (admin audit view).
	ListByUser(ctx context.Context, workspaceID, userEmail string, limit int) ([]RevokedSession, error)
	// CleanupExpired deletes revocation rows whose JWTs have expired naturally.
	CleanupExpired(ctx context.Context) (int64, error)
}

type revokedSessionsRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewRevokedSessionsRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) RevokedSessionRepository {
	return &revokedSessionsRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *revokedSessionsRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *revokedSessionsRepo) Revoke(ctx context.Context, s *RevokedSession) error {
	ctx, span := r.tracer.Start(ctx, "revoked_sessions.repository.Revoke")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var wsParam any
	if s.WorkspaceID != "" {
		wsParam = s.WorkspaceID
	}
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO revoked_sessions
            (workspace_id, jti, user_email, reason, revoked_by, expires_at)
        VALUES ($1::uuid, $2, $3, $4, $5, $6)
        ON CONFLICT (jti) DO NOTHING`,
		wsParam, s.JTI, s.UserEmail, s.Reason, s.RevokedBy, s.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (r *revokedSessionsRepo) IsRevoked(ctx context.Context, jti string) (bool, error) {
	ctx, span := r.tracer.Start(ctx, "revoked_sessions.repository.IsRevoked")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if jti == "" {
		return false, nil
	}
	var n int
	err := r.db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM revoked_sessions
         WHERE jti = $1 AND expires_at > NOW()`, jti).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("is revoked: %w", err)
	}
	return n > 0, nil
}

func (r *revokedSessionsRepo) ListByUser(ctx context.Context, workspaceID, userEmail string, limit int) ([]RevokedSession, error) {
	ctx, span := r.tracer.Start(ctx, "revoked_sessions.repository.ListByUser")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
        SELECT id::text, COALESCE(workspace_id::text,''), jti, user_email,
               COALESCE(reason,''), COALESCE(revoked_by,''), revoked_at, expires_at
          FROM revoked_sessions
         WHERE user_email = $1 AND ($2 = '' OR workspace_id::text = $2)
           AND expires_at > NOW()
         ORDER BY revoked_at DESC
         LIMIT $3`, userEmail, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RevokedSession
	for rows.Next() {
		var s RevokedSession
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.JTI, &s.UserEmail,
			&s.Reason, &s.RevokedBy, &s.RevokedAt, &s.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *revokedSessionsRepo) CleanupExpired(ctx context.Context) (int64, error) {
	ctx, span := r.tracer.Start(ctx, "revoked_sessions.repository.CleanupExpired")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `DELETE FROM revoked_sessions WHERE expires_at <= NOW()`)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
