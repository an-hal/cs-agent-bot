package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

// MasterDataRepository describes data access for the Master Data view (over clients).
type MasterDataRepository interface {
	List(ctx context.Context, filter entity.MasterDataFilter) ([]entity.MasterData, int64, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.MasterData, error)
	Create(ctx context.Context, workspaceID string, m *entity.MasterData) (*entity.MasterData, error)
	Patch(ctx context.Context, workspaceID, id string, patch MasterDataPatch) (*entity.MasterData, error)
	HardDelete(ctx context.Context, workspaceID, id string) error
	Stats(ctx context.Context, workspaceID string) (*MasterDataStats, error)
	Attention(ctx context.Context, workspaceID, search string, offset, limit int) ([]entity.MasterData, int64, error)
	Query(ctx context.Context, workspaceID string, conds []QueryCondition, limit int) ([]entity.MasterData, int64, error)
	Transition(ctx context.Context, workspaceID, id string, newStage string, coreUpdates MasterDataPatch, customUpdates map[string]any) (*entity.MasterData, *entity.MasterData, error)
	MergeCustomFields(ctx context.Context, workspaceID, id string, updates map[string]any) error
	BulkUpdateDaysToExpiry(ctx context.Context, workspaceID string) (int64, error)
	BulkMarkOverdue(ctx context.Context, workspaceID string) (int64, error)
	// ExistingCompanyIDs returns the subset of given company_ids that already
	// exist for this workspace. Used by import-preview for dedup and by the
	// client-create path when FE wants to show a "duplicate" warning pre-submit.
	ExistingCompanyIDs(ctx context.Context, workspaceID string, companyIDs []string) (map[string]string, error)
}

// MasterDataPatch carries optional core column updates.
type MasterDataPatch struct {
	CompanyName     *string
	Stage           *string
	PICName         *string
	PICNickname     *string
	PICRole         *string
	PICWA           *string
	PICEmail        *string
	OwnerName       *string
	OwnerWA         *string
	OwnerTelegramID *string
	BotActive       *bool
	Blacklisted     *bool
	SequenceStatus  *string
	SnoozeUntil     *time.Time
	SnoozeReason    *string
	RiskFlag        *string
	ContractStart   *time.Time
	ContractEnd     *time.Time
	ContractMonths  *int
	PaymentStatus   *string
	PaymentTerms    *string
	FinalPrice      *int64
	LastPaymentDate *time.Time
	Renewed         *bool
	Notes           *string
	CustomFields    map[string]any
}

// MasterDataStats is the response shape for /master-data/stats.
type MasterDataStats struct {
	Total           int64            `json:"total"`
	ByStage         map[string]int64 `json:"by_stage"`
	TotalRevenue    int64            `json:"total_revenue"`
	HighRisk        int64            `json:"high_risk"`
	OverduePayment  int64            `json:"overdue_payment"`
	Expiring30d     int64            `json:"expiring_30d"`
	BotActiveCount  int64            `json:"bot_active"`
}

// QueryCondition is one row of the flexible workflow query DSL.
type QueryCondition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

// AllowedQueryOps is the whitelist for /query.
var AllowedQueryOps = map[string]bool{
	"=": true, "!=": true, ">": true, ">=": true, "<": true, "<=": true,
	"in": true, "between": true, "contains": true,
}

type masterDataRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewMasterDataRepo constructs a MasterDataRepository.
func NewMasterDataRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) MasterDataRepository {
	return &masterDataRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *masterDataRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const masterDataColumns = `
    id::text, workspace_id::text, company_id, company_name, stage,
    COALESCE(pic_name,''), COALESCE(pic_nickname,''), COALESCE(pic_role,''), COALESCE(pic_wa,''), COALESCE(pic_email,''),
    COALESCE(owner_name,''), COALESCE(owner_wa,''), COALESCE(owner_telegram_id,''),
    bot_active, blacklisted, COALESCE(sequence_status,'ACTIVE'), snooze_until, COALESCE(snooze_reason,''),
    COALESCE(risk_flag,'None'),
    contract_start, contract_end, COALESCE(contract_months,0), days_to_expiry,
    COALESCE(payment_status,'Pending'), COALESCE(payment_terms,''),
    COALESCE(final_price,0)::bigint, last_payment_date,
    COALESCE(billing_period,'monthly'), quantity, unit_price, COALESCE(currency,'IDR'),
    last_interaction_date, COALESCE(notes,''),
    COALESCE(custom_fields,'{}'::jsonb), created_at, updated_at`

func scanMasterData(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.MasterData, error) {
	var (
		m            entity.MasterData
		customRaw    []byte
		contractMon  sql.NullInt32
	)
	_ = contractMon
	err := scanner.Scan(
		&m.ID, &m.WorkspaceID, &m.CompanyID, &m.CompanyName, &m.Stage,
		&m.PICName, &m.PICNickname, &m.PICRole, &m.PICWA, &m.PICEmail,
		&m.OwnerName, &m.OwnerWA, &m.OwnerTelegramID,
		&m.BotActive, &m.Blacklisted, &m.SequenceStatus, &m.SnoozeUntil, &m.SnoozeReason,
		&m.RiskFlag,
		&m.ContractStart, &m.ContractEnd, &m.ContractMonths, &m.DaysToExpiry,
		&m.PaymentStatus, &m.PaymentTerms,
		&m.FinalPrice, &m.LastPaymentDate,
		&m.BillingPeriod, &m.Quantity, &m.UnitPrice, &m.Currency,
		&m.LastInteractionDate, &m.Notes,
		&customRaw, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(customRaw) > 0 {
		if err := json.Unmarshal(customRaw, &m.CustomFields); err != nil {
			return nil, fmt.Errorf("unmarshal custom_fields: %w", err)
		}
	}
	if m.CustomFields == nil {
		m.CustomFields = map[string]any{}
	}
	return &m, nil
}

func (r *masterDataRepo) baseListConds(filter entity.MasterDataFilter) sq.Sqlizer {
	conds := sq.And{}
	if len(filter.WorkspaceIDs) == 1 {
		conds = append(conds, sq.Expr("workspace_id::text = ?", filter.WorkspaceIDs[0]))
	} else if len(filter.WorkspaceIDs) > 1 {
		ph := make([]string, 0, len(filter.WorkspaceIDs))
		args := make([]any, 0, len(filter.WorkspaceIDs))
		for _, id := range filter.WorkspaceIDs {
			ph = append(ph, "?")
			args = append(args, id)
		}
		conds = append(conds, sq.Expr("workspace_id::text IN ("+strings.Join(ph, ",")+")", args...))
	}
	if len(filter.Stages) > 0 {
		conds = append(conds, sq.Eq{"stage": filter.Stages})
	}
	if filter.Search != "" {
		s := "%" + filter.Search + "%"
		conds = append(conds, sq.Or{
			sq.Expr("company_name ILIKE ?", s),
			sq.Expr("pic_name ILIKE ?", s),
			sq.Expr("company_id ILIKE ?", s),
		})
	}
	if filter.RiskFlag != "" {
		conds = append(conds, sq.Eq{"risk_flag": filter.RiskFlag})
	}
	if filter.BotActive != nil {
		conds = append(conds, sq.Eq{"bot_active": *filter.BotActive})
	}
	if filter.PaymentStatus != "" {
		conds = append(conds, sq.Eq{"payment_status": filter.PaymentStatus})
	}
	if filter.ExpiryWithin > 0 {
		conds = append(conds, sq.Expr("days_to_expiry BETWEEN 0 AND ?", filter.ExpiryWithin))
	}
	return conds
}

func (r *masterDataRepo) List(ctx context.Context, filter entity.MasterDataFilter) ([]entity.MasterData, int64, error) {
	ctx, span := r.tracer.Start(ctx, "master_data.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	sortBy := filter.SortBy
	switch sortBy {
	case "company_name", "stage", "created_at", "contract_end", "final_price":
	default:
		sortBy = "updated_at"
	}
	sortDir := strings.ToUpper(filter.SortDir)
	if sortDir != "ASC" {
		sortDir = "DESC"
	}

	conds := r.baseListConds(filter)

	q := database.PSQL.
		Select(masterDataColumns).
		From("master_data").
		Where(conds).
		OrderBy(sortBy + " " + sortDir).
		Limit(uint64(limit)).
		Offset(uint64(offset))

	query, args, err := q.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build list: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query master_data: %w", err)
	}
	defer rows.Close()

	var out []entity.MasterData
	for rows.Next() {
		m, err := scanMasterData(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		out = append(out, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	countQuery, countArgs, err := database.PSQL.
		Select("COUNT(*)").From("master_data").Where(conds).ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count: %w", err)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}
	return out, total, nil
}

func (r *masterDataRepo) GetByID(ctx context.Context, workspaceID, id string) (*entity.MasterData, error) {
	ctx, span := r.tracer.Start(ctx, "master_data.repository.GetByID")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(masterDataColumns).
		From("master_data").
		Where(sq.And{
			sq.Expr("workspace_id::text = ?", workspaceID),
			sq.Expr("id::text = ?", id),
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}
	m, err := scanMasterData(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query: %w", err)
	}
	return m, nil
}

func (r *masterDataRepo) Create(ctx context.Context, workspaceID string, m *entity.MasterData) (*entity.MasterData, error) {
	ctx, span := r.tracer.Start(ctx, "master_data.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if m.Stage == "" {
		m.Stage = entity.StageLead
	}
	if m.SequenceStatus == "" {
		m.SequenceStatus = entity.SeqStatusActive
	}
	if m.RiskFlag == "" {
		m.RiskFlag = entity.RiskNone
	}
	if m.PaymentStatus == "" {
		m.PaymentStatus = "Pending"
	}
	if m.CustomFields == nil {
		m.CustomFields = map[string]any{}
	}
	cfRaw, err := json.Marshal(m.CustomFields)
	if err != nil {
		return nil, fmt.Errorf("marshal custom_fields: %w", err)
	}

	// Phase 6 split: clients holds CRM core; bot_active/sequence_status/snooze*
	// live in client_message_state. We insert the core row, then upsert the
	// companion message-state row so reads via the master_data view return the
	// caller's intended values.
	// Phase 5 generic billing: persist billing_period/quantity/unit_price/
	// currency in clients (defaults applied via COALESCE so an empty
	// string/nil from the caller doesn't override the column default).
	insertQ := `
INSERT INTO clients (
    workspace_id, company_id, company_name, stage,
    pic_name, pic_nickname, pic_role, pic_wa, pic_email,
    owner_name, owner_wa, owner_telegram_id,
    blacklisted,
    risk_flag_text, contract_start, contract_end, contract_months,
    payment_status, payment_terms, final_price, last_payment_date,
    billing_period, quantity, unit_price, currency,
    notes, custom_fields,
    activation_date
) VALUES (
    $1::uuid, $2, $3, $4,
    NULLIF($5,''), NULLIF($6,''), NULLIF($7,''), $8, NULLIF($9,''),
    $10, NULLIF($11,''), $12,
    $13,
    $14, $15, $16, $17,
    $18, NULLIF($19,''), $20, $21,
    COALESCE(NULLIF($22,''), 'monthly'), $23, $24, COALESCE(NULLIF($25,''), 'IDR'),
    NULLIF($26,''), $27::jsonb,
    COALESCE($15, NOW()::date)
)
RETURNING master_id::text`

	var newID string
	row := r.db.QueryRowContext(ctx, insertQ,
		workspaceID, m.CompanyID, m.CompanyName, m.Stage,
		m.PICName, m.PICNickname, m.PICRole, m.PICWA, m.PICEmail,
		m.OwnerName, m.OwnerWA, m.OwnerTelegramID,
		m.Blacklisted,
		m.RiskFlag, m.ContractStart, m.ContractEnd, m.ContractMonths,
		m.PaymentStatus, m.PaymentTerms, m.FinalPrice, m.LastPaymentDate,
		m.BillingPeriod, m.Quantity, m.UnitPrice, m.Currency,
		m.Notes, string(cfRaw),
	)
	if err := row.Scan(&newID); err != nil {
		return nil, fmt.Errorf("insert client: %w", err)
	}

	// Upsert companion message-state row with last_interaction_date so
	// dashboard creates can seed it. ON CONFLICT keeps the call idempotent
	// in case a previous partial insert left a stale row.
	if _, err := r.db.ExecContext(ctx, `
INSERT INTO client_message_state (
    master_id, workspace_id,
    bot_active, sequence_status, snooze_until, snooze_reason,
    last_interaction_date
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, NULLIF($6,''), $7)
ON CONFLICT (master_id) DO UPDATE SET
    bot_active = EXCLUDED.bot_active,
    sequence_status = EXCLUDED.sequence_status,
    snooze_until = EXCLUDED.snooze_until,
    snooze_reason = EXCLUDED.snooze_reason,
    last_interaction_date = COALESCE(EXCLUDED.last_interaction_date, client_message_state.last_interaction_date),
    updated_at = NOW()`,
		newID, workspaceID,
		m.BotActive, m.SequenceStatus, m.SnoozeUntil, m.SnoozeReason,
		m.LastInteractionDate,
	); err != nil {
		return nil, fmt.Errorf("upsert client_message_state: %w", err)
	}
	return r.GetByID(ctx, workspaceID, newID)
}

func (r *masterDataRepo) Patch(ctx context.Context, workspaceID, id string, patch MasterDataPatch) (*entity.MasterData, error) {
	ctx, span := r.tracer.Start(ctx, "master_data.repository.Patch")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	upd := database.PSQL.Update("clients").Where(sq.And{
		sq.Expr("workspace_id::text = ?", workspaceID),
		sq.Expr("master_id::text = ?", id),
	})
	dirty := false
	setStr := func(col string, p *string) {
		if p != nil {
			upd = upd.Set(col, *p)
			dirty = true
		}
	}
	setBool := func(col string, p *bool) {
		if p != nil {
			upd = upd.Set(col, *p)
			dirty = true
		}
	}
	setStr("company_name", patch.CompanyName)
	setStr("stage", patch.Stage)
	setStr("pic_name", patch.PICName)
	setStr("pic_nickname", patch.PICNickname)
	setStr("pic_role", patch.PICRole)
	setStr("pic_wa", patch.PICWA)
	setStr("pic_email", patch.PICEmail)
	setStr("owner_name", patch.OwnerName)
	setStr("owner_wa", patch.OwnerWA)
	setStr("owner_telegram_id", patch.OwnerTelegramID)
	// bot_active/sequence_status/snooze_* now live in client_message_state.
	// Collected here, applied after the clients UPDATE in a separate UPSERT so
	// the patch is atomic-per-table and idempotent on missing companion rows.
	cmsUpd := database.PSQL.Update("client_message_state").Where(sq.And{
		sq.Expr("workspace_id::text = ?", workspaceID),
		sq.Expr("master_id::text = ?", id),
	})
	cmsDirty := false
	if patch.BotActive != nil {
		cmsUpd = cmsUpd.Set("bot_active", *patch.BotActive)
		cmsDirty = true
	}
	if patch.SequenceStatus != nil {
		cmsUpd = cmsUpd.Set("sequence_status", *patch.SequenceStatus)
		cmsDirty = true
	}
	if patch.SnoozeUntil != nil {
		cmsUpd = cmsUpd.Set("snooze_until", *patch.SnoozeUntil)
		cmsDirty = true
	}
	if patch.SnoozeReason != nil {
		cmsUpd = cmsUpd.Set("snooze_reason", *patch.SnoozeReason)
		cmsDirty = true
	}
	setBool("blacklisted", patch.Blacklisted)
	setStr("risk_flag_text", patch.RiskFlag)
	if patch.ContractStart != nil {
		upd = upd.Set("contract_start", *patch.ContractStart)
		dirty = true
	}
	if patch.ContractEnd != nil {
		upd = upd.Set("contract_end", *patch.ContractEnd)
		dirty = true
	}
	if patch.ContractMonths != nil {
		upd = upd.Set("contract_months", *patch.ContractMonths)
		dirty = true
	}
	setStr("payment_status", patch.PaymentStatus)
	setStr("payment_terms", patch.PaymentTerms)
	if patch.FinalPrice != nil {
		upd = upd.Set("final_price", *patch.FinalPrice)
		dirty = true
	}
	if patch.LastPaymentDate != nil {
		upd = upd.Set("last_payment_date", *patch.LastPaymentDate)
		dirty = true
	}
	// renewed column dropped — value goes to custom_fields. If the patch
	// includes it, the caller must put it under CustomFields["renewed"]
	// instead of using PatchRequest.Renewed (which is now a no-op).
	setStr("notes", patch.Notes)
	if patch.CustomFields != nil {
		raw, err := json.Marshal(patch.CustomFields)
		if err != nil {
			return nil, fmt.Errorf("marshal custom: %w", err)
		}
		// Deep-merge via JSONB concat.
		upd = upd.Set("custom_fields", sq.Expr("COALESCE(custom_fields,'{}'::jsonb) || ?::jsonb", string(raw)))
		dirty = true
	}
	if !dirty && !cmsDirty {
		return r.GetByID(ctx, workspaceID, id)
	}

	if dirty {
		query, args, err := upd.ToSql()
		if err != nil {
			return nil, fmt.Errorf("build patch: %w", err)
		}
		if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
			return nil, fmt.Errorf("exec patch: %w", err)
		}
	}

	if cmsDirty {
		// Try UPDATE first; if no companion row exists yet (legacy data), insert
		// one and re-apply. master_data view's COALESCE keeps reads safe even
		// if cms is missing, but writes need an actual row.
		query, args, err := cmsUpd.ToSql()
		if err != nil {
			return nil, fmt.Errorf("build cms patch: %w", err)
		}
		res, err := r.db.ExecContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("exec cms patch: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			if _, err := r.db.ExecContext(ctx,
				`INSERT INTO client_message_state (master_id, workspace_id) VALUES ($1::uuid, $2::uuid) ON CONFLICT (master_id) DO NOTHING`,
				id, workspaceID,
			); err != nil {
				return nil, fmt.Errorf("seed cms row: %w", err)
			}
			if _, err := r.db.ExecContext(ctx, query, args...); err != nil {
				return nil, fmt.Errorf("retry cms patch: %w", err)
			}
		}
	}
	return r.GetByID(ctx, workspaceID, id)
}

func (r *masterDataRepo) HardDelete(ctx context.Context, workspaceID, id string) error {
	ctx, span := r.tracer.Start(ctx, "master_data.repository.HardDelete")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		"DELETE FROM clients WHERE workspace_id::text = $1 AND master_id::text = $2",
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

func (r *masterDataRepo) Stats(ctx context.Context, workspaceID string) (*MasterDataStats, error) {
	ctx, span := r.tracer.Start(ctx, "master_data.repository.Stats")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	stats := &MasterDataStats{ByStage: map[string]int64{}}

	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM master_data WHERE workspace_id::text = $1",
		workspaceID,
	).Scan(&stats.Total); err != nil {
		return nil, fmt.Errorf("count total: %w", err)
	}

	rows, err := r.db.QueryContext(ctx,
		"SELECT stage, COUNT(*) FROM master_data WHERE workspace_id::text = $1 GROUP BY stage",
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("by stage: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var stage string
		var n int64
		if err := rows.Scan(&stage, &n); err != nil {
			return nil, err
		}
		stats.ByStage[stage] = n
	}

	_ = r.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(final_price),0)::bigint FROM master_data WHERE workspace_id::text = $1",
		workspaceID,
	).Scan(&stats.TotalRevenue)

	_ = r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM master_data WHERE workspace_id::text = $1 AND risk_flag = 'High'",
		workspaceID,
	).Scan(&stats.HighRisk)

	_ = r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM master_data WHERE workspace_id::text = $1 AND payment_status IN ('Overdue','Terlambat','Belum bayar')",
		workspaceID,
	).Scan(&stats.OverduePayment)

	_ = r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM master_data WHERE workspace_id::text = $1 AND days_to_expiry BETWEEN 0 AND 30",
		workspaceID,
	).Scan(&stats.Expiring30d)

	_ = r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM master_data WHERE workspace_id::text = $1 AND bot_active = TRUE",
		workspaceID,
	).Scan(&stats.BotActiveCount)

	return stats, nil
}

func (r *masterDataRepo) Attention(ctx context.Context, workspaceID, search string, offset, limit int) ([]entity.MasterData, int64, error) {
	ctx, span := r.tracer.Start(ctx, "master_data.repository.Attention")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	conds := sq.And{
		sq.Expr("workspace_id::text = ?", workspaceID),
		sq.Or{
			sq.Eq{"risk_flag": "High"},
			sq.Expr("payment_status IN ('Overdue','Terlambat','Belum bayar')"),
			sq.Expr("days_to_expiry BETWEEN 0 AND 30"),
		},
	}
	if search != "" {
		s := "%" + search + "%"
		conds = append(conds, sq.Or{
			sq.Expr("company_name ILIKE ?", s),
			sq.Expr("pic_name ILIKE ?", s),
		})
	}
	q := database.PSQL.
		Select(masterDataColumns).From("master_data").Where(conds).
		OrderBy("updated_at DESC").
		Limit(uint64(limit)).Offset(uint64(offset))

	query, args, err := q.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build attention: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query attention: %w", err)
	}
	defer rows.Close()
	var out []entity.MasterData
	for rows.Next() {
		m, err := scanMasterData(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *m)
	}
	cq, ca, _ := database.PSQL.Select("COUNT(*)").From("master_data").Where(conds).ToSql()
	var total int64
	_ = r.db.QueryRowContext(ctx, cq, ca...).Scan(&total)
	return out, total, nil
}

// Query implements the flexible workflow query. The op set is whitelisted and
// JSONB key paths are sanitized so the only attacker-controlled input that
// reaches SQL is via parameter binding.
func (r *masterDataRepo) Query(ctx context.Context, workspaceID string, conds []QueryCondition, limit int) ([]entity.MasterData, int64, error) {
	ctx, span := r.tracer.Start(ctx, "master_data.repository.Query")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	where := sq.And{sq.Expr("workspace_id::text = ?", workspaceID)}
	for _, c := range conds {
		clause, err := buildConditionClause(c)
		if err != nil {
			return nil, 0, err
		}
		where = append(where, clause)
	}

	q := database.PSQL.
		Select(masterDataColumns).
		From("master_data").
		Where(where).
		Limit(uint64(limit))

	query, args, err := q.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build query: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()
	var out []entity.MasterData
	for rows.Next() {
		m, err := scanMasterData(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *m)
	}
	return out, int64(len(out)), nil
}

func (r *masterDataRepo) Transition(
	ctx context.Context,
	workspaceID, id string,
	newStage string,
	coreUpdates MasterDataPatch,
	customUpdates map[string]any,
) (*entity.MasterData, *entity.MasterData, error) {
	ctx, span := r.tracer.Start(ctx, "master_data.repository.Transition")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	prev, err := r.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot prev: %w", err)
	}
	if prev == nil {
		return nil, nil, errors.New("not found")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		"UPDATE clients SET stage = $1 WHERE workspace_id::text = $2 AND master_id::text = $3",
		newStage, workspaceID, id,
	); err != nil {
		return nil, nil, fmt.Errorf("update stage: %w", err)
	}
	if customUpdates != nil {
		raw, err := json.Marshal(customUpdates)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE clients SET custom_fields = COALESCE(custom_fields,'{}'::jsonb) || $1::jsonb WHERE workspace_id::text = $2 AND master_id::text = $3",
			string(raw), workspaceID, id,
		); err != nil {
			return nil, nil, fmt.Errorf("merge custom: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}
	curr, err := r.GetByID(ctx, workspaceID, id)
	if err != nil {
		return prev, nil, err
	}
	return prev, curr, nil
}

func (r *masterDataRepo) MergeCustomFields(ctx context.Context, workspaceID, id string, updates map[string]any) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	raw, err := json.Marshal(updates)
	if err != nil {
		return fmt.Errorf("marshal custom_fields: %w", err)
	}
	_, err = r.db.ExecContext(ctx,
		"UPDATE clients SET custom_fields = COALESCE(custom_fields,'{}'::jsonb) || $1::jsonb WHERE workspace_id::text = $2 AND master_id::text = $3",
		string(raw), workspaceID, id,
	)
	if err != nil {
		return fmt.Errorf("merge custom_fields: %w", err)
	}
	return nil
}

func (r *masterDataRepo) BulkUpdateDaysToExpiry(ctx context.Context, workspaceID string) (int64, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		`UPDATE clients SET days_to_expiry = GREATEST(EXTRACT(DAY FROM (contract_end - NOW()))::int, 0)
		 WHERE workspace_id::text = $1 AND stage = 'CLIENT' AND contract_end IS NOT NULL`,
		workspaceID,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk update days_to_expiry: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *masterDataRepo) BulkMarkOverdue(ctx context.Context, workspaceID string) (int64, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		`UPDATE clients SET payment_status = 'Overdue'
		 WHERE workspace_id::text = $1
		   AND stage = 'CLIENT'
		   AND payment_status IN ('Pending','Menunggu')
		   AND contract_end IS NOT NULL
		   AND contract_end < NOW()`,
		workspaceID,
	)
	if err != nil {
		return 0, fmt.Errorf("bulk mark overdue: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// buildConditionClause sanitizes and converts one QueryCondition.
// SAFETY: only allow ops in the whitelist; JSONB key path must be [A-Za-z0-9_.];
// every value flows through parameter binding.
func buildConditionClause(c QueryCondition) (sq.Sqlizer, error) {
	if !AllowedQueryOps[strings.ToLower(c.Op)] {
		return nil, fmt.Errorf("query op %q not allowed", c.Op)
	}
	op := strings.ToLower(c.Op)

	field := c.Field
	if !isSafeFieldName(field) {
		return nil, fmt.Errorf("query field %q is invalid", field)
	}

	expr, isJSON := fieldExpression(field)
	switch op {
	case "between":
		arr, ok := c.Value.([]any)
		if !ok || len(arr) != 2 {
			return nil, fmt.Errorf("between requires a 2-element array")
		}
		castExpr := castForJSON(expr, isJSON, arr[0])
		return sq.Expr(castExpr+" BETWEEN ? AND ?", arr[0], arr[1]), nil
	case "in":
		arr, ok := c.Value.([]any)
		if !ok || len(arr) == 0 {
			return nil, fmt.Errorf("in requires a non-empty array")
		}
		ph := make([]string, len(arr))
		for i := range arr {
			ph[i] = "?"
		}
		castExpr := castForJSON(expr, isJSON, arr[0])
		return sq.Expr(castExpr+" IN ("+strings.Join(ph, ",")+")", arr...), nil
	case "contains":
		castExpr := castForJSON(expr, isJSON, "")
		return sq.Expr(castExpr+" ILIKE ?", "%"+fmt.Sprint(c.Value)+"%"), nil
	default:
		castExpr := castForJSON(expr, isJSON, c.Value)
		return sq.Expr(castExpr+" "+op+" ?", c.Value), nil
	}
}

func isSafeFieldName(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r == '_' || r == '.' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// fieldExpression maps a logical field name to a SQL expression and reports
// whether the result is a JSONB text extraction.
func fieldExpression(field string) (string, bool) {
	if strings.HasPrefix(field, "custom_fields.") {
		key := strings.TrimPrefix(field, "custom_fields.")
		// JSONB key extraction: custom_fields->>'key'
		// We've already validated `field` chars; key is safe.
		return "custom_fields->>'" + key + "'", true
	}
	return field, false
}

func castForJSON(expr string, isJSON bool, sample any) string {
	if !isJSON {
		return expr
	}
	switch sample.(type) {
	case float64, int, int32, int64:
		return "(" + expr + ")::numeric"
	case bool:
		return "(" + expr + ")::boolean"
	default:
		return expr
	}
}

// ExistingCompanyIDs returns a map of company_id -> master_data.id for any of
// the given company_ids that already exist in this workspace. Uses a single
// ANY query so it's O(1) DB round-trips regardless of input size (bounded at
// 1000 ids for safety).
func (r *masterDataRepo) ExistingCompanyIDs(ctx context.Context, workspaceID string, companyIDs []string) (map[string]string, error) {
	ctx, span := r.tracer.Start(ctx, "master_data.repository.ExistingCompanyIDs")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	out := map[string]string{}
	if len(companyIDs) == 0 {
		return out, nil
	}
	if len(companyIDs) > 1000 {
		companyIDs = companyIDs[:1000]
	}
	rows, err := r.db.QueryContext(ctx, `
        SELECT company_id, id::text
          FROM master_data
         WHERE workspace_id::text = $1
           AND company_id = ANY($2)`,
		workspaceID, pq.Array(companyIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("existing company_ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, id string
		if err := rows.Scan(&cid, &id); err != nil {
			return nil, err
		}
		out[cid] = id
	}
	return out, rows.Err()
}
