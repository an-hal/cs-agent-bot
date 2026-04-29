package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

// ClientRepository defines the interface for client data operations.
type ClientRepository interface {
	GetAll(ctx context.Context) ([]entity.Client, error)
	GetByID(ctx context.Context, companyID string) (*entity.Client, error)
	GetByWANumber(ctx context.Context, waNumber string) (*entity.Client, error)
	GetByCompanyID(ctx context.Context, companyID string) (*entity.Client, error)
	GetLatestInvoice(ctx context.Context, companyID string) (*entity.Invoice, error)
	CreateClient(ctx context.Context, client entity.Client) error
	UpdateInvoiceReminderFlags(ctx context.Context, companyID string, flags map[string]bool) error
	UpdatePaymentStatus(ctx context.Context, companyID, status string) error
	GetAllByWorkspace(ctx context.Context, workspaceSlug string) ([]entity.Client, error)
	GetAllByWorkspaceIDs(ctx context.Context, workspaceIDs []string) ([]entity.Client, error)
	CountByFilter(ctx context.Context, filter entity.ClientFilter) (int64, error)
	FetchByFilter(ctx context.Context, filter entity.ClientFilter, p pagination.Params) ([]entity.Client, error)
	UpdateClientFields(ctx context.Context, companyID string, fields map[string]interface{}) error
	UpdateClientCustomFields(ctx context.Context, companyID string, fields map[string]any) error
}

type clientRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewClientRepo creates a new ClientRepository backed by PostgreSQL.
func NewClientRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ClientRepository {
	return &clientRepo{
		db:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

// withTimeout returns a context with the configured query timeout.
// If the timeout is zero, the original context is returned with a no-op cancel.
func (r *clientRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// clientColumns lists every field of entity.Client in the order expected by scanClient.
// bot_active and sequence_cs live in client_message_state (joined as cms); everything
// else comes from clients (aliased c).
const clientColumns = `c.company_id, c.company_name, c.pic_name, COALESCE(c.pic_wa, '') as pic_wa, ` +
	`c.owner_name, c.owner_wa, COALESCE(cms.owner_telegram_id, '') as owner_telegram_id, ` +
	`c.contract_months, c.contract_start, COALESCE(c.contract_end, '9999-12-31'::date) as contract_end, ` +
	`c.activation_date, COALESCE(c.payment_status, 'Paid') as payment_status, ` +
	`COALESCE(cms.bot_active, true) as bot_active, ` +
	`COALESCE(cms.blacklisted, false) as blacklisted, ` +
	`COALESCE(cms.sequence_cs, 'ACTIVE') as sequence_cs, ` +
	`COALESCE(c.pic_email, '') as pic_email, COALESCE(c.pic_role, '') as pic_role, ` +
	`COALESCE(c.payment_terms, '') as payment_terms, ` +
	`COALESCE(c.final_price, 0) as final_price, c.last_payment_date, ` +
	`COALESCE(c.notes, '') as notes, c.created_at, ` +
	`COALESCE(cms.ae_assigned, false) as ae_assigned, ` +
	`COALESCE(cms.backup_owner_telegram_id, '') as backup_owner_telegram_id, ` +
	`COALESCE(cms.ae_telegram_id, '') as ae_telegram_id, ` +
	`COALESCE(c.workspace_id::text, '') as workspace_id, ` +
	`COALESCE(c.custom_fields, '{}') as custom_fields, ` +
	`COALESCE(c.billing_period, 'monthly') as billing_period, c.quantity, c.unit_price, ` +
	`COALESCE(c.currency, 'IDR') as currency`

// clientFromClause is the standard FROM ... JOIN combo. Use as
// `FROM ` + clientFromClause + ` WHERE ...`.
const clientFromClause = `clients c LEFT JOIN client_message_state cms ON cms.master_id = c.master_id`

// invoiceColumns lists every column read from the invoices table.
const invoiceColumns = "invoice_id, company_id, due_date, amount, payment_status"

// scanClient scans a single client row from the current position of the provided scanner.
func scanClient(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.Client, error) {
	var c entity.Client
	var customFieldsRaw []byte
	err := scanner.Scan(
		&c.CompanyID,
		&c.CompanyName,
		&c.PICName,
		&c.PICWA,
		&c.OwnerName,
		&c.OwnerWA,
		&c.OwnerTelegramID,
		&c.ContractMonths,
		&c.ContractStart,
		&c.ContractEnd,
		&c.ActivationDate,
		&c.PaymentStatus,
		&c.BotActive,
		&c.Blacklisted,
		&c.SequenceCS,
		&c.PICEmail,
		&c.PICRole,
		&c.PaymentTerms,
		&c.FinalPrice,
		&c.LastPaymentDate,
		&c.Notes,
		&c.CreatedAt,
		&c.AEAssigned,
		&c.BackupOwnerTelegramID,
		&c.AETelegramID,
		&c.WorkspaceID,
		&customFieldsRaw,
		&c.BillingPeriod,
		&c.Quantity,
		&c.UnitPrice,
		&c.Currency,
	)
	if err == nil && len(customFieldsRaw) > 0 {
		_ = json.Unmarshal(customFieldsRaw, &c.CustomFields)
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetAll returns all active (non-blacklisted, bot-enabled) clients ordered by company_id.
func (r *clientRepo) GetAll(ctx context.Context) ([]entity.Client, error) {
	ctx, span := r.tracer.Start(ctx, "client.repository.GetAll")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(clientColumns).
		From("clients c").
		LeftJoin("client_message_state cms ON cms.master_id = c.master_id").
		From("clients c").
		LeftJoin("client_message_state cms ON cms.master_id = c.master_id").
		Where(sq.And{
			sq.Eq{"cms.blacklisted": false},
			sq.Eq{"cms.bot_active": true},
		}).
		OrderBy("company_id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetAll: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query GetAll: %w", err)
	}
	defer rows.Close()

	var clients []entity.Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, fmt.Errorf("scan client row: %w", err)
		}
		clients = append(clients, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate client rows: %w", err)
	}

	return clients, nil
}

// GetByID returns a single client by company_id.
func (r *clientRepo) GetByID(ctx context.Context, companyID string) (*entity.Client, error) {
	ctx, span := r.tracer.Start(ctx, "client.repository.GetByID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(clientColumns).
		From("clients c").
		LeftJoin("client_message_state cms ON cms.master_id = c.master_id").
		Where(sq.Eq{"c.company_id": companyID}).
		From("clients c").
		LeftJoin("client_message_state cms ON cms.master_id = c.master_id").
		Where(sq.Eq{"c.company_id": companyID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetByID: %w", err)
	}

	row := r.db.QueryRowContext(ctx, query, args...)
	c, err := scanClient(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query GetByID: %w", err)
	}

	return c, nil
}

// GetByWANumber returns a single client matching the given PIC WA number.
func (r *clientRepo) GetByWANumber(ctx context.Context, waNumber string) (*entity.Client, error) {
	ctx, span := r.tracer.Start(ctx, "client.repository.GetByWANumber")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(clientColumns).
		From("clients c").
		LeftJoin("client_message_state cms ON cms.master_id = c.master_id").
		Where(sq.Eq{"c.pic_wa": waNumber}).
		From("clients c").
		LeftJoin("client_message_state cms ON cms.master_id = c.master_id").
		Where(sq.Eq{"c.pic_wa": waNumber}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetByWANumber: %w", err)
	}

	row := r.db.QueryRowContext(ctx, query, args...)
	c, err := scanClient(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("client not found for WA number: %s", waNumber)
		}
		return nil, fmt.Errorf("query GetByWANumber: %w", err)
	}

	return c, nil
}

// GetByCompanyID returns a single client by company_id. It is an alias for GetByID.
func (r *clientRepo) GetByCompanyID(ctx context.Context, companyID string) (*entity.Client, error) {
	return r.GetByID(ctx, companyID)
}

// GetLatestInvoice returns the most recent invoice for a given company.
func (r *clientRepo) GetLatestInvoice(ctx context.Context, companyID string) (*entity.Invoice, error) {
	ctx, span := r.tracer.Start(ctx, "client.repository.GetLatestInvoice")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(invoiceColumns).
		From("invoices").
		Where(sq.Eq{"company_id": companyID}).
		OrderBy("created_at DESC").
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query GetLatestInvoice: %w", err)
	}

	var inv entity.Invoice
	err = r.db.QueryRowContext(ctx, query, args...).Scan(
		&inv.InvoiceID,
		&inv.CompanyID,
		&inv.DueDate,
		&inv.Amount,
		&inv.PaymentStatus,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no invoice found for company: %s", companyID)
		}
		return nil, fmt.Errorf("query GetLatestInvoice: %w", err)
	}

	return &inv, nil
}

// UpdatePaymentStatus updates the payment_status and sets last_interaction_date to NOW().
func (r *clientRepo) UpdatePaymentStatus(ctx context.Context, companyID, status string) error {
	ctx, span := r.tracer.Start(ctx, "client.repository.UpdatePaymentStatus")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Update("clients").
		Set("payment_status", status).
		Where(sq.Eq{"company_id": companyID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query UpdatePaymentStatus: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec UpdatePaymentStatus: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", companyID)
	}

	return nil
}

// validReminderFlags maps accepted flag column names to a placeholder value.
// Only these keys are accepted in the flags map; unknown keys are silently ignored.
var validReminderFlags = map[string]struct{}{
	"pre14_sent":  {},
	"pre7_sent":   {},
	"pre3_sent":   {},
	"post1_sent":  {},
	"post4_sent":  {},
	"post8_sent":  {},
	"post15_sent": {},
}

// UpdateInvoiceReminderFlags dynamically updates invoice reminder boolean flags for a client.
// Only the keys listed in validReminderFlags are applied; unknown keys are ignored.
func (r *clientRepo) UpdateInvoiceReminderFlags(ctx context.Context, companyID string, flags map[string]bool) error {
	ctx, span := r.tracer.Start(ctx, "client.repository.UpdateInvoiceReminderFlags")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	sets := make(map[string]interface{}, len(flags))
	for key, val := range flags {
		if _, ok := validReminderFlags[key]; ok {
			sets[key] = val
		}
	}

	if len(sets) == 0 {
		return nil
	}

	query, args, err := database.PSQL.
		Update("client_flags").
		SetMap(sets).
		Where(sq.Eq{"company_id": companyID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query UpdateInvoiceReminderFlags: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec UpdateInvoiceReminderFlags: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", companyID)
	}

	return nil
}

// CreateClient inserts a new client or updates all updatable fields on conflict with company_id.
func (r *clientRepo) CreateClient(ctx context.Context, client entity.Client) error {
	ctx, span := r.tracer.Start(ctx, "client.repository.CreateClient")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	upsertSuffix := "ON CONFLICT (company_id) DO UPDATE SET " +
		"company_name = EXCLUDED.company_name, " +
		"pic_name = EXCLUDED.pic_name, " +
		"pic_wa = EXCLUDED.pic_wa, " +
		"pic_email = EXCLUDED.pic_email, " +
		"pic_role = EXCLUDED.pic_role, " +
		"owner_name = EXCLUDED.owner_name, " +
		"owner_wa = EXCLUDED.owner_wa, " +
		"owner_telegram_id = EXCLUDED.owner_telegram_id, " +
		"payment_terms = EXCLUDED.payment_terms, " +
		"contract_months = EXCLUDED.contract_months, " +
		"contract_start = EXCLUDED.contract_start, " +
		"contract_end = EXCLUDED.contract_end, " +
		"activation_date = EXCLUDED.activation_date, " +
		"final_price = EXCLUDED.final_price, " +
		"notes = EXCLUDED.notes, " +
		"payment_status = EXCLUDED.payment_status, " +
		"last_payment_date = EXCLUDED.last_payment_date, " +
		"bot_active = EXCLUDED.bot_active, " +
		"blacklisted = EXCLUDED.blacklisted, " +
		"sequence_cs = EXCLUDED.sequence_cs, " +
		"ae_assigned = EXCLUDED.ae_assigned, " +
		"backup_owner_telegram_id = EXCLUDED.backup_owner_telegram_id, " +
		"ae_telegram_id = EXCLUDED.ae_telegram_id, " +
		"custom_fields = EXCLUDED.custom_fields, " +
		"workspace_id = EXCLUDED.workspace_id"

	customFieldsJSON, _ := json.Marshal(client.CustomFields)

	query, args, err := database.PSQL.
		Insert("clients").
		Columns(
			"company_id", "company_name", "pic_name", "pic_wa", "pic_email", "pic_role",
			"owner_name", "owner_wa", "owner_telegram_id",
			"payment_terms",
			"contract_months", "contract_start", "contract_end",
			"activation_date", "payment_status",
			"final_price", "notes",
			"last_payment_date",
			"bot_active", "blacklisted",
			"sequence_cs",
			"ae_assigned", "backup_owner_telegram_id", "ae_telegram_id",
			"custom_fields",
			"workspace_id",
		).
		Values(
			client.CompanyID, client.CompanyName, client.PICName, client.PICWA, client.PICEmail, client.PICRole,
			client.OwnerName, client.OwnerWA, client.OwnerTelegramID,
			client.PaymentTerms,
			client.ContractMonths, client.ContractStart, client.ContractEnd,
			client.ActivationDate, client.PaymentStatus,
			client.FinalPrice, client.Notes,
			client.LastPaymentDate,
			client.BotActive, client.Blacklisted,
			client.SequenceCS,
			client.AEAssigned, client.BackupOwnerTelegramID, client.AETelegramID,
			customFieldsJSON,
			client.WorkspaceID,
		).
		Suffix(upsertSuffix).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query CreateClient: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec CreateClient: %w", err)
	}

	return nil
}

// GetAllByWorkspace returns all non-blacklisted clients for a workspace slug,
// with JOINed latest invoice and conversation state.
func (r *clientRepo) GetAllByWorkspace(ctx context.Context, workspaceSlug string) ([]entity.Client, error) {
	ctx, span := r.tracer.Start(ctx, "client.repository.GetAllByWorkspace")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := fmt.Sprintf(
		"SELECT %s FROM clients c LEFT JOIN client_message_state cms ON cms.master_id = c.master_id WHERE cms.blacklisted = false AND c.workspace_id = (SELECT id FROM workspaces WHERE slug = $1) ORDER BY c.company_id",
		clientColumns,
	)

	rows, err := r.db.QueryContext(ctx, query, workspaceSlug)
	if err != nil {
		return nil, fmt.Errorf("query GetAllByWorkspace: %w", err)
	}
	defer rows.Close()

	var clients []entity.Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, fmt.Errorf("scan client row: %w", err)
		}
		clients = append(clients, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate client rows: %w", err)
	}
	return clients, nil
}

// GetAllByWorkspaceIDs returns all non-blacklisted clients for multiple workspace IDs (holding mode).
func (r *clientRepo) GetAllByWorkspaceIDs(ctx context.Context, workspaceIDs []string) ([]entity.Client, error) {
	ctx, span := r.tracer.Start(ctx, "client.repository.GetAllByWorkspaceIDs")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query := fmt.Sprintf(
		"SELECT %s FROM clients c LEFT JOIN client_message_state cms ON cms.master_id = c.master_id WHERE cms.blacklisted = false AND c.workspace_id::text = ANY($1) ORDER BY c.company_id",
		clientColumns,
	)
	rows, err := r.db.QueryContext(ctx, query, pq.Array(workspaceIDs))
	if err != nil {
		return nil, fmt.Errorf("query GetAllByWorkspaceIDs: %w", err)
	}
	defer rows.Close()

	var clients []entity.Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, fmt.Errorf("scan client row: %w", err)
		}
		clients = append(clients, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate client rows: %w", err)
	}
	return clients, nil
}

// CountByFilter returns the total number of non-blacklisted clients matching the filter.
func (r *clientRepo) CountByFilter(ctx context.Context, filter entity.ClientFilter) (int64, error) {
	ctx, span := r.tracer.Start(ctx, "client.repository.CountByFilter")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	where := buildClientFilter(filter)

	query, args, err := database.PSQL.
		Select("COUNT(*)").
		From("clients").
		Where(where).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count query CountByFilter: %w", err)
	}

	var count int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count CountByFilter: %w", err)
	}
	return count, nil
}

// FetchByFilter returns paginated non-blacklisted clients matching the filter.
func (r *clientRepo) FetchByFilter(ctx context.Context, filter entity.ClientFilter, p pagination.Params) ([]entity.Client, error) {
	ctx, span := r.tracer.Start(ctx, "client.repository.FetchByFilter")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	where := buildClientFilter(filter)

	query, args, err := database.PSQL.
		Select(clientColumns).
		From("clients c").
		LeftJoin("client_message_state cms ON cms.master_id = c.master_id").
		From("clients c").
		LeftJoin("client_message_state cms ON cms.master_id = c.master_id").
		Where(where).
		OrderBy("c.company_id").
		OrderBy("c.company_id").
		Limit(uint64(p.Limit)).
		Offset(uint64(p.Offset)).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query FetchByWorkspaceID: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query FetchByWorkspaceID: %w", err)
	}
	defer rows.Close()

	var clients []entity.Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, fmt.Errorf("scan client row: %w", err)
		}
		clients = append(clients, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate client rows: %w", err)
	}
	return clients, nil
}

// buildClientFilter constructs a sq.And condition for client queries with
// workspace and optional filters. Columns prefixed with `c.` live on
// `clients`; `cms.` columns live on `client_message_state` and require the
// LEFT JOIN to be present in the FROM clause. `segment` and `plan_type`
// filters are no-ops since those columns moved to custom_fields.
// buildClientFilter constructs a sq.And condition for client queries with
// workspace and optional filters. Columns prefixed with `c.` live on
// `clients`; `cms.` columns live on `client_message_state` and require the
// LEFT JOIN to be present in the FROM clause. `segment` and `plan_type`
// filters are no-ops since those columns moved to custom_fields.
func buildClientFilter(filter entity.ClientFilter) sq.And {
	where := sq.And{
		sq.Eq{"cms.blacklisted": false},
	}
	if len(filter.WorkspaceIDs) > 0 {
		where = append(where, sq.Expr("c.workspace_id::text = ANY(?)", pq.Array(filter.WorkspaceIDs)))
		where = append(where, sq.Expr("c.workspace_id::text = ANY(?)", pq.Array(filter.WorkspaceIDs)))
	}
	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		where = append(where, sq.Or{
			sq.ILike{"c.company_name": pattern},
			sq.ILike{"c.pic_name": pattern},
			sq.ILike{"c.pic_wa": pattern},
			sq.ILike{"c.pic_email": pattern},
			sq.ILike{"c.owner_name": pattern},
			sq.ILike{"c.company_id": pattern},
			sq.ILike{"c.company_name": pattern},
			sq.ILike{"c.pic_name": pattern},
			sq.ILike{"c.pic_wa": pattern},
			sq.ILike{"c.pic_email": pattern},
			sq.ILike{"c.owner_name": pattern},
			sq.ILike{"c.company_id": pattern},
		})
	}
	if filter.Segment != "" {
		where = append(where, sq.Expr("custom_fields->>'segment' = ?", filter.Segment))
	}
	if filter.PaymentStatus != "" {
		where = append(where, sq.Eq{"c.payment_status": filter.PaymentStatus})
		where = append(where, sq.Eq{"c.payment_status": filter.PaymentStatus})
	}
	if filter.SequenceCS != "" {
		where = append(where, sq.Eq{"cms.sequence_cs": filter.SequenceCS})
	}
	if filter.PlanType != "" {
		where = append(where, sq.Expr("custom_fields->>'plan_type' = ?", filter.PlanType))
	}
	if filter.BotActive != nil {
		where = append(where, sq.Eq{"cms.bot_active": *filter.BotActive})
		where = append(where, sq.Eq{"cms.bot_active": *filter.BotActive})
	}
	// Segment and PlanType moved to custom_fields. Use JSONB filter via
	// trigger_rules / FE-side query if these are needed.
	_ = filter.Segment
	_ = filter.PlanType
	// Segment and PlanType moved to custom_fields. Use JSONB filter via
	// trigger_rules / FE-side query if these are needed.
	_ = filter.Segment
	_ = filter.PlanType
	return where
}

// UpdateClientCustomFields merges the provided key-value pairs into clients.custom_fields JSONB.
func (r *clientRepo) UpdateClientCustomFields(ctx context.Context, companyID string, fields map[string]any) error {
	ctx, span := r.tracer.Start(ctx, "client.repository.UpdateClientCustomFields")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if len(fields) == 0 {
		return nil
	}

	raw, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshal custom fields: %w", err)
	}

	result, err := r.db.ExecContext(ctx,
		"UPDATE clients SET custom_fields = COALESCE(custom_fields, '{}'::jsonb) || $1::jsonb WHERE company_id = $2",
		string(raw), companyID,
	)
	if err != nil {
		return fmt.Errorf("exec UpdateClientCustomFields: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", companyID)
	}

	return nil
}

// validUpdateColumns lists columns that are safe to update via the dashboard API.
var validUpdateColumns = map[string]bool{
	"pic_name": true, "pic_wa": true, "pic_email": true, "pic_role": true,
	"owner_name": true, "owner_wa": true, "owner_telegram_id": true,
	"payment_terms": true, "contract_months": true,
	"contract_start": true, "contract_end": true,
	"activation_date": true, "payment_status": true,
	"final_price": true,
	"notes":       true,
	"bot_active":  true, "blacklisted": true,
	"sequence_cs": true,
}

// UpdateClientFields dynamically updates specified fields for a client.
func (r *clientRepo) UpdateClientFields(ctx context.Context, companyID string, fields map[string]interface{}) error {
	ctx, span := r.tracer.Start(ctx, "client.repository.UpdateClientFields")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if len(fields) == 0 {
		return nil
	}

	safeFields := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		if !validUpdateColumns[k] {
			return fmt.Errorf("field not allowed for update: %s", k)
		}
		safeFields[k] = v
	}

	query, args, err := database.PSQL.
		Update("clients").
		SetMap(safeFields).
		Where(sq.Eq{"company_id": companyID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query UpdateClientFields: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec UpdateClientFields: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", companyID)
	}
	return nil
}
