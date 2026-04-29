package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// ClientContactRepository persists per-stage internal & client-side PICs.
// See context/new/multi-stage-pic-spec.md for the data model.
type ClientContactRepository interface {
	List(ctx context.Context, workspaceID, masterID string, filter ClientContactFilter) ([]entity.ClientContact, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.ClientContact, error)
	Create(ctx context.Context, c *entity.ClientContact) (*entity.ClientContact, error)
	Patch(ctx context.Context, workspaceID, id string, patch ClientContactPatch) (*entity.ClientContact, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// ClientContactFilter narrows a List query. Empty values are no-op.
type ClientContactFilter struct {
	Stage     string
	Kind      string
	OnlyPrimary bool
}

// ClientContactPatch is the optional-fields update payload.
type ClientContactPatch struct {
	Stage      *string
	Kind       *string
	Role       *string
	Name       *string
	WA         *string
	Email      *string
	TelegramID *string
	IsPrimary  *bool
	Notes      *string
}

type clientContactRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewClientContactRepo constructs the repository.
func NewClientContactRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ClientContactRepository {
	return &clientContactRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *clientContactRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const ccColumns = `id::text, workspace_id::text, master_id::text,
    stage, kind, COALESCE(role,''),
    name, COALESCE(wa,''), COALESCE(email,''), COALESCE(telegram_id,''),
    is_primary, COALESCE(notes,''),
    created_at, updated_at`

func scanContact(scanner interface {
	Scan(...any) error
}) (*entity.ClientContact, error) {
	var c entity.ClientContact
	if err := scanner.Scan(
		&c.ID, &c.WorkspaceID, &c.MasterDataID,
		&c.Stage, &c.Kind, &c.Role,
		&c.Name, &c.WA, &c.Email, &c.TelegramID,
		&c.IsPrimary, &c.Notes,
		&c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *clientContactRepo) List(ctx context.Context, workspaceID, masterID string, filter ClientContactFilter) ([]entity.ClientContact, error) {
	ctx, span := r.tracer.Start(ctx, "client_contact.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{
		sq.Expr("workspace_id::text = ?", workspaceID),
		sq.Expr("master_id::text = ?", masterID),
	}
	if filter.Stage != "" {
		conds = append(conds, sq.Eq{"stage": filter.Stage})
	}
	if filter.Kind != "" {
		conds = append(conds, sq.Eq{"kind": filter.Kind})
	}
	if filter.OnlyPrimary {
		conds = append(conds, sq.Eq{"is_primary": true})
	}
	q, args, err := database.PSQL.Select(ccColumns).
		From("client_contacts").Where(conds).
		OrderBy("stage", "kind", "is_primary DESC", "created_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query list: %w", err)
	}
	defer rows.Close()
	out := make([]entity.ClientContact, 0)
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (r *clientContactRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.ClientContact, error) {
	ctx, span := r.tracer.Start(ctx, "client_contact.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()
	q := "SELECT " + ccColumns + " FROM client_contacts WHERE workspace_id::text = $1 AND id::text = $2"
	row := r.db.QueryRowContext(ctx, q, workspaceID, id)
	c, err := scanContact(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}
	return c, nil
}

// Create inserts a new contact. When IsPrimary=true, any existing primary
// for the same (master_id, stage, kind) is auto-demoted in the same
// transaction so the partial unique index doesn't trip.
func (r *clientContactRepo) Create(ctx context.Context, c *entity.ClientContact) (*entity.ClientContact, error) {
	ctx, span := r.tracer.Start(ctx, "client_contact.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if c.WorkspaceID == "" || c.MasterDataID == "" || c.Stage == "" || c.Kind == "" || c.Name == "" {
		return nil, errors.New("workspace_id, master_id, stage, kind, name required")
	}
	if !entity.IsValidContactKind(c.Kind) {
		return nil, fmt.Errorf("invalid kind %q (must be internal | client_side)", c.Kind)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if c.IsPrimary {
		if _, err := tx.ExecContext(ctx, `
UPDATE client_contacts SET is_primary = FALSE, updated_at = NOW()
WHERE master_id::text = $1 AND stage = $2 AND kind = $3 AND is_primary = TRUE`,
			c.MasterDataID, c.Stage, c.Kind,
		); err != nil {
			return nil, fmt.Errorf("demote prior primary: %w", err)
		}
	}

	row := tx.QueryRowContext(ctx, `
INSERT INTO client_contacts (
    workspace_id, master_id, stage, kind, role,
    name, wa, email, telegram_id, is_primary, notes
) VALUES ($1::uuid, $2::uuid, $3, $4, NULLIF($5,''),
    $6, NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), $10, NULLIF($11,''))
RETURNING `+ccColumns,
		c.WorkspaceID, c.MasterDataID, c.Stage, c.Kind, c.Role,
		c.Name, c.WA, c.Email, c.TelegramID, c.IsPrimary, c.Notes,
	)
	saved, err := scanContact(row)
	if err != nil {
		return nil, fmt.Errorf("insert contact: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return saved, nil
}

// Patch updates partial fields. If IsPrimary is being toggled to true, the
// previous primary in the same (stage, kind) is auto-demoted.
func (r *clientContactRepo) Patch(ctx context.Context, workspaceID, id string, patch ClientContactPatch) (*entity.ClientContact, error) {
	ctx, span := r.tracer.Start(ctx, "client_contact.repository.Patch")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// If promoting to primary, demote whoever currently is primary in the
	// same (stage, kind). We need the current row first to know stage/kind
	// because patch may not include them.
	var (
		stage, kind string
		masterID    string
	)
	if err := tx.QueryRowContext(ctx,
		`SELECT stage, kind, master_id::text FROM client_contacts WHERE workspace_id::text = $1 AND id::text = $2`,
		workspaceID, id).Scan(&stage, &kind, &masterID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup before patch: %w", err)
	}
	if patch.Stage != nil {
		stage = *patch.Stage
	}
	if patch.Kind != nil {
		kind = *patch.Kind
	}
	if patch.IsPrimary != nil && *patch.IsPrimary {
		if _, err := tx.ExecContext(ctx, `
UPDATE client_contacts SET is_primary = FALSE, updated_at = NOW()
WHERE master_id::text = $1 AND stage = $2 AND kind = $3 AND is_primary = TRUE AND id::text <> $4`,
			masterID, stage, kind, id,
		); err != nil {
			return nil, fmt.Errorf("demote prior primary: %w", err)
		}
	}

	upd := database.PSQL.Update("client_contacts").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Expr("id::text = ?", id),
		})
	dirty := false
	setStr := func(col string, p *string) {
		if p != nil {
			upd = upd.Set(col, *p)
			dirty = true
		}
	}
	setStr("stage", patch.Stage)
	setStr("kind", patch.Kind)
	setStr("role", patch.Role)
	setStr("name", patch.Name)
	setStr("wa", patch.WA)
	setStr("email", patch.Email)
	setStr("telegram_id", patch.TelegramID)
	setStr("notes", patch.Notes)
	if patch.IsPrimary != nil {
		upd = upd.Set("is_primary", *patch.IsPrimary)
		dirty = true
	}
	if !dirty {
		return r.GetByID(ctx, workspaceID, id)
	}
	q, args, err := upd.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build patch: %w", err)
	}
	if _, err := tx.ExecContext(ctx, q, args...); err != nil {
		return nil, fmt.Errorf("exec patch: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return r.GetByID(ctx, workspaceID, id)
}

func (r *clientContactRepo) Delete(ctx context.Context, workspaceID, id string) error {
	ctx, span := r.tracer.Start(ctx, "client_contact.repository.Delete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM client_contacts WHERE workspace_id::text = $1 AND id::text = $2`,
		workspaceID, id)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("not found")
	}
	return nil
}

