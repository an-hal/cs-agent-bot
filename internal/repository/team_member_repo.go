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

// TeamMemberRepository is CRUD for team_members.
type TeamMemberRepository interface {
	List(ctx context.Context, filter TeamMemberFilter) ([]entity.TeamMember, int64, error)
	GetByID(ctx context.Context, id string) (*entity.TeamMember, error)
	GetByEmail(ctx context.Context, email string) (*entity.TeamMember, error)
	GetByInviteToken(ctx context.Context, token string) (*entity.TeamMember, error)
	Create(ctx context.Context, m *entity.TeamMember) (*entity.TeamMember, error)
	Update(ctx context.Context, id string, patch TeamMemberPatch) (*entity.TeamMember, error)
	Delete(ctx context.Context, id string) error
	Summary(ctx context.Context, workspaceID string) (TeamMemberSummary, error)
}

// TeamMemberFilter drives list queries.
type TeamMemberFilter struct {
	WorkspaceID string // optional — filter to members assigned to this workspace
	Status      string
	RoleID      string
	Search      string
	Offset      int
	Limit       int
	SortBy      string
	SortDir     string
}

// TeamMemberPatch is a partial member update. Nil pointer = leave unchanged.
type TeamMemberPatch struct {
	Name          *string
	Department    *string
	AvatarColor   *string
	Status        *string
	RoleID        *string
	InviteToken   *string
	InviteExpires *time.Time
	JoinedAt      *time.Time
	UserID        *string
}

// TeamMemberSummary is a counts-by-status roll-up.
type TeamMemberSummary struct {
	Total    int64
	Active   int64
	Pending  int64
	Inactive int64
}

type teamMemberRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewTeamMemberRepo constructs a postgres-backed team member repository.
func NewTeamMemberRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) TeamMemberRepository {
	return &teamMemberRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *teamMemberRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const tmColumns = `tm.id::text, COALESCE(tm.user_id::text,''), tm.name, tm.email, tm.initials,
    tm.role_id::text, tm.status, tm.department, tm.avatar_color,
    COALESCE(tm.invite_token,''), tm.invite_expires,
    COALESCE(tm.invited_by::text,''), tm.joined_at, tm.last_active_at,
    tm.created_at, tm.updated_at`

func scanTeamMember(s interface{ Scan(dest ...interface{}) error }) (*entity.TeamMember, error) {
	var m entity.TeamMember
	if err := s.Scan(
		&m.ID, &m.UserID, &m.Name, &m.Email, &m.Initials,
		&m.RoleID, &m.Status, &m.Department, &m.AvatarColor,
		&m.InviteToken, &m.InviteExpires,
		&m.InvitedBy, &m.JoinedAt, &m.LastActiveAt,
		&m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *teamMemberRepo) List(ctx context.Context, f TeamMemberFilter) ([]entity.TeamMember, int64, error) {
	ctx, span := r.tracer.Start(ctx, "team_member.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	base := database.PSQL.Select(tmColumns).From("team_members tm")
	countBase := database.PSQL.Select("COUNT(DISTINCT tm.id)").From("team_members tm")

	if f.WorkspaceID != "" {
		base = base.Join("member_workspace_assignments mwa ON mwa.member_id = tm.id").
			Where(sq.Expr("mwa.workspace_id::text = ?", f.WorkspaceID))
		countBase = countBase.Join("member_workspace_assignments mwa ON mwa.member_id = tm.id").
			Where(sq.Expr("mwa.workspace_id::text = ?", f.WorkspaceID))
	}
	if f.Status != "" {
		base = base.Where(sq.Eq{"tm.status": f.Status})
		countBase = countBase.Where(sq.Eq{"tm.status": f.Status})
	}
	if f.RoleID != "" {
		base = base.Where(sq.Expr("tm.role_id::text = ?", f.RoleID))
		countBase = countBase.Where(sq.Expr("tm.role_id::text = ?", f.RoleID))
	}
	if f.Search != "" {
		like := "%" + strings.ToLower(f.Search) + "%"
		cond := sq.Or{
			sq.Expr("LOWER(tm.name) LIKE ?", like),
			sq.Expr("LOWER(tm.email) LIKE ?", like),
			sq.Expr("LOWER(tm.department) LIKE ?", like),
		}
		base = base.Where(cond)
		countBase = countBase.Where(cond)
	}

	sortCol := "tm.name"
	switch f.SortBy {
	case "email":
		sortCol = "tm.email"
	case "joined_at":
		sortCol = "tm.joined_at"
	case "last_active_at":
		sortCol = "tm.last_active_at"
	}
	dir := "ASC"
	if strings.EqualFold(f.SortDir, "desc") {
		dir = "DESC"
	}
	base = base.OrderBy(sortCol + " " + dir)
	if f.Limit > 0 {
		base = base.Limit(uint64(f.Limit)).Offset(uint64(f.Offset))
	}

	query, args, err := base.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build list: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	var out []entity.TeamMember
	for rows.Next() {
		m, err := scanTeamMember(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		out = append(out, *m)
	}
	if rerr := rows.Err(); rerr != nil {
		return nil, 0, rerr
	}

	var total int64
	cquery, cargs, cerr := countBase.ToSql()
	if cerr != nil {
		return nil, 0, fmt.Errorf("build count: %w", cerr)
	}
	if err := r.db.QueryRowContext(ctx, cquery, cargs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	return out, total, nil
}

func (r *teamMemberRepo) getBy(ctx context.Context, where sq.Sqlizer) (*entity.TeamMember, error) {
	query, args, err := database.PSQL.Select(tmColumns).From("team_members tm").Where(where).ToSql()
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	m, err := scanTeamMember(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query: %w", err)
	}
	return m, nil
}

func (r *teamMemberRepo) GetByID(ctx context.Context, id string) (*entity.TeamMember, error) {
	ctx, span := r.tracer.Start(ctx, "team_member.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()
	return r.getBy(ctx, sq.Expr("tm.id::text = ?", id))
}

func (r *teamMemberRepo) GetByEmail(ctx context.Context, email string) (*entity.TeamMember, error) {
	ctx, span := r.tracer.Start(ctx, "team_member.repository.GetByEmail")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()
	return r.getBy(ctx, sq.Eq{"tm.email": strings.ToLower(email)})
}

func (r *teamMemberRepo) GetByInviteToken(ctx context.Context, token string) (*entity.TeamMember, error) {
	ctx, span := r.tracer.Start(ctx, "team_member.repository.GetByInviteToken")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()
	return r.getBy(ctx, sq.Eq{"tm.invite_token": token})
}

func (r *teamMemberRepo) Create(ctx context.Context, m *entity.TeamMember) (*entity.TeamMember, error) {
	ctx, span := r.tracer.Start(ctx, "team_member.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := `
        INSERT INTO team_members
            (name, email, initials, role_id, status, department, avatar_color,
             invite_token, invite_expires, invited_by)
        VALUES ($1, $2, $3, $4::uuid, $5, $6, $7, NULLIF($8,''), $9, NULLIF($10,'')::uuid)
        RETURNING ` + tmColumns

	out, err := scanTeamMember(r.db.QueryRowContext(ctx, query,
		m.Name, strings.ToLower(m.Email), m.Initials, m.RoleID, m.Status,
		m.Department, m.AvatarColor, m.InviteToken, m.InviteExpires, m.InvitedBy,
	))
	if err != nil {
		return nil, fmt.Errorf("insert team member: %w", err)
	}
	return out, nil
}

func (r *teamMemberRepo) Update(ctx context.Context, id string, patch TeamMemberPatch) (*entity.TeamMember, error) {
	ctx, span := r.tracer.Start(ctx, "team_member.repository.Update")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	upd := database.PSQL.Update("team_members tm").Where(sq.Expr("tm.id::text = ?", id))
	dirty := false
	if patch.Name != nil {
		upd = upd.Set("name", *patch.Name)
		dirty = true
	}
	if patch.Department != nil {
		upd = upd.Set("department", *patch.Department)
		dirty = true
	}
	if patch.AvatarColor != nil {
		upd = upd.Set("avatar_color", *patch.AvatarColor)
		dirty = true
	}
	if patch.Status != nil {
		upd = upd.Set("status", *patch.Status)
		dirty = true
	}
	if patch.RoleID != nil {
		upd = upd.Set("role_id", sq.Expr("?::uuid", *patch.RoleID))
		dirty = true
	}
	if patch.InviteToken != nil {
		if *patch.InviteToken == "" {
			upd = upd.Set("invite_token", nil)
		} else {
			upd = upd.Set("invite_token", *patch.InviteToken)
		}
		dirty = true
	}
	if patch.InviteExpires != nil {
		upd = upd.Set("invite_expires", *patch.InviteExpires)
		dirty = true
	}
	if patch.JoinedAt != nil {
		upd = upd.Set("joined_at", *patch.JoinedAt)
		dirty = true
	}
	if patch.UserID != nil {
		if *patch.UserID == "" {
			upd = upd.Set("user_id", nil)
		} else {
			upd = upd.Set("user_id", sq.Expr("?::uuid", *patch.UserID))
		}
		dirty = true
	}
	if !dirty {
		return r.GetByID(ctx, id)
	}
	upd = upd.Set("updated_at", sq.Expr("NOW()")).Suffix("RETURNING " + tmColumns)

	query, args, err := upd.ToSql()
	if err != nil {
		return nil, fmt.Errorf("build update: %w", err)
	}
	out, err := scanTeamMember(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("update: %w", err)
	}
	return out, nil
}

func (r *teamMemberRepo) Delete(ctx context.Context, id string) error {
	ctx, span := r.tracer.Start(ctx, "team_member.repository.Delete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx, `DELETE FROM team_members WHERE id::text = $1`, id)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTeamNotFound
	}
	return nil
}

func (r *teamMemberRepo) Summary(ctx context.Context, workspaceID string) (TeamMemberSummary, error) {
	ctx, span := r.tracer.Start(ctx, "team_member.repository.Summary")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := `
        SELECT
            COUNT(*) FILTER (WHERE TRUE) AS total,
            COUNT(*) FILTER (WHERE tm.status = 'active')   AS active,
            COUNT(*) FILTER (WHERE tm.status = 'pending')  AS pending,
            COUNT(*) FILTER (WHERE tm.status = 'inactive') AS inactive
        FROM team_members tm`
	args := []any{}
	if workspaceID != "" {
		query += ` JOIN member_workspace_assignments mwa ON mwa.member_id = tm.id
                   WHERE mwa.workspace_id::text = $1`
		args = append(args, workspaceID)
	}

	var s TeamMemberSummary
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&s.Total, &s.Active, &s.Pending, &s.Inactive); err != nil {
		return s, fmt.Errorf("summary: %w", err)
	}
	return s, nil
}
