package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/lib/pq"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// safeString returns the string representation of row[idx] or "" if out of bounds or nil.
// Shared helper used by other repository files that still read from Google Sheets.
func safeString(row []interface{}, idx int) string {
	if idx >= len(row) || row[idx] == nil {
		return ""
	}
	return fmt.Sprintf("%v", row[idx])
}

// ClientRepository defines the interface for client data operations.
type ClientRepository interface {
	GetAll(ctx context.Context) ([]entity.Client, error)
	GetByID(ctx context.Context, companyID string) (*entity.Client, error)
	GetByWANumber(ctx context.Context, waNumber string) (*entity.Client, error)
	GetByCompanyID(ctx context.Context, companyID string) (*entity.Client, error)
	GetLatestInvoice(ctx context.Context, companyID string) (*entity.Invoice, error)
	UpdateLastInteraction(ctx context.Context, companyID string, t time.Time) error
	CreateClient(ctx context.Context, client entity.Client) error
	UpdateInvoiceReminderFlags(ctx context.Context, companyID string, flags map[string]bool) error
	UpdatePaymentStatus(ctx context.Context, companyID, status string) error
	GetAllByWorkspace(ctx context.Context, workspaceSlug string) ([]entity.Client, error)
	GetAllByWorkspaceIDs(ctx context.Context, workspaceIDs []string) ([]entity.Client, error)
	UpdateClientFields(ctx context.Context, companyID string, fields map[string]interface{}) error
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

// clientColumns lists every column in the clients table in a fixed order.
// This constant is used for SELECT queries to avoid SELECT *.
// Nullable columns that use Go pointer types (e.g. owner_wa, last_interaction_date) have no COALESCE.
// Nullable columns that use Go value types still use COALESCE for zero-value defaults.
const clientColumns = `company_id, company_name, pic_name, pic_wa, owner_name, owner_wa, owner_telegram_id, COALESCE(segment, '') as segment, contract_months, contract_start, contract_end, activation_date, COALESCE(payment_status, 'Paid') as payment_status, COALESCE(nps_score, 0) as nps_score, COALESCE(usage_score, 0) as usage_score, COALESCE(bot_active, true) as bot_active, COALESCE(blacklisted, false) as blacklisted, COALESCE(renewed, false) as renewed, COALESCE(rejected, false) as rejected, COALESCE(quotation_link, '') as quotation_link, COALESCE(sequence_cs, 'ACTIVE') as sequence_cs, COALESCE(cross_sell_rejected, false) as cross_sell_rejected, COALESCE(cross_sell_interested, false) as cross_sell_interested, COALESCE(checkin_replied, false) as checkin_replied, COALESCE(response_status, 'Pending') as response_status, COALESCE(pre14_sent, false) as pre14_sent, COALESCE(pre7_sent, false) as pre7_sent, COALESCE(pre3_sent, false) as pre3_sent, COALESCE(post1_sent, false) as post1_sent, COALESCE(post4_sent, false) as post4_sent, COALESCE(post8_sent, false) as post8_sent, COALESCE(post15_sent, false) as post15_sent, last_interaction_date, COALESCE(pic_email, '') as pic_email, COALESCE(pic_role, '') as pic_role, COALESCE(hc_size, '') as hc_size, COALESCE(plan_type, '') as plan_type, COALESCE(payment_terms, '') as payment_terms, COALESCE(final_price, 0) as final_price, last_payment_date, COALESCE(notes, '') as notes, cross_sell_resume_date, renewal_date, created_at, COALESCE(churn_reason, '') as churn_reason, COALESCE(wa_undeliverable, false) as wa_undeliverable, COALESCE(feature_update_sent, false) as feature_update_sent, COALESCE(days_since_cs_last_sent, 0) as days_since_cs_last_sent, COALESCE(first_time_discount_pct, 0) as first_time_discount_pct, COALESCE(next_discount_pct_manual, 0) as next_discount_pct_manual, quotation_link_expires, COALESCE(ae_assigned, false) as ae_assigned, COALESCE(usage_score_avg_30d, 0) as usage_score_avg_30d, COALESCE(backup_owner_telegram_id, '') as backup_owner_telegram_id, COALESCE(ae_telegram_id, '') as ae_telegram_id, COALESCE(bd_prospect_id, '') as bd_prospect_id, COALESCE(risk_flag, false) as risk_flag, COALESCE(workspace_id::text, '') as workspace_id`

// invoiceColumns lists every column read from the invoices table.
const invoiceColumns = "invoice_id, company_id, due_date, amount, payment_status"

// scanClient scans a single client row from the current position of the provided scanner.
func scanClient(scanner interface{ Scan(dest ...interface{}) error }) (*entity.Client, error) {
	var c entity.Client
	err := scanner.Scan(
		&c.CompanyID,
		&c.CompanyName,
		&c.PICName,
		&c.PICWA,
		&c.OwnerName,
		&c.OwnerWA,
		&c.OwnerTelegramID,
		&c.Segment,
		&c.ContractMonths,
		&c.ContractStart,
		&c.ContractEnd,
		&c.ActivationDate,
		&c.PaymentStatus,
		&c.NPSScore,
		&c.UsageScore,
		&c.BotActive,
		&c.Blacklisted,
		&c.Renewed,
		&c.Rejected,
		&c.QuotationLink,
		&c.SequenceCS,
		&c.CrossSellRejected,
		&c.CrossSellInterested,
		&c.CheckinReplied,
		&c.ResponseStatus,
		&c.Pre14Sent,
		&c.Pre7Sent,
		&c.Pre3Sent,
		&c.Post1Sent,
		&c.Post4Sent,
		&c.Post8Sent,
		&c.Post15Sent,
		&c.LastInteractionDate,
		&c.PICEmail,
		&c.PICRole,
		&c.HCSize,
		&c.PlanType,
		&c.PaymentTerms,
		&c.FinalPrice,
		&c.LastPaymentDate,
		&c.Notes,
		&c.CrossSellResumeDate,
		&c.RenewalDate,
		&c.CreatedAt,
		&c.ChurnReason,
		&c.WAUndeliverable,
		&c.FeatureUpdateSent,
		&c.DaysSinceCSLastSent,
		&c.FirstTimeDiscountPct,
		&c.NextDiscountPctManual,
		&c.QuotationLinkExpires,
		&c.AEAssigned,
		&c.UsageScoreAvg30d,
		&c.BackupOwnerTelegramID,
		&c.AETelegramID,
		&c.BDProspectID,
		&c.RiskFlag,
		&c.WorkspaceID,
	)
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
		From("clients").
		Where(sq.And{
			sq.Eq{"blacklisted": false},
			sq.Eq{"bot_active": true},
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
		From("clients").
		Where(sq.Eq{"company_id": companyID}).
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
		From("clients").
		Where(sq.Eq{"pic_wa": waNumber}).
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
		SetMap(map[string]interface{}{
			"payment_status":       status,
			"last_interaction_date": sq.Expr("NOW()"),
		}).
		Where(sq.Eq{"company_id": companyID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query UpdatePaymentStatus: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec UpdatePaymentStatus: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
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
		Update("clients").
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

	rowsAffected, _ := result.RowsAffected()
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

	upsertSuffix := fmt.Sprintf(
		"ON CONFLICT (company_id) DO UPDATE SET "+
			"company_name = EXCLUDED.company_name, "+
			"pic_name = EXCLUDED.pic_name, "+
			"pic_wa = EXCLUDED.pic_wa, "+
			"pic_email = EXCLUDED.pic_email, "+
			"pic_role = EXCLUDED.pic_role, "+
			"hc_size = EXCLUDED.hc_size, "+
			"owner_name = EXCLUDED.owner_name, "+
			"owner_wa = EXCLUDED.owner_wa, "+
			"owner_telegram_id = EXCLUDED.owner_telegram_id, "+
			"segment = EXCLUDED.segment, "+
			"plan_type = EXCLUDED.plan_type, "+
			"payment_terms = EXCLUDED.payment_terms, "+
			"contract_months = EXCLUDED.contract_months, "+
			"contract_start = EXCLUDED.contract_start, "+
			"contract_end = EXCLUDED.contract_end, "+
			"activation_date = EXCLUDED.activation_date, "+
			"final_price = EXCLUDED.final_price, "+
			"quotation_link = EXCLUDED.quotation_link, "+
			"renewal_date = EXCLUDED.renewal_date, "+
			"notes = EXCLUDED.notes, "+
			"payment_status = EXCLUDED.payment_status, "+
			"last_payment_date = EXCLUDED.last_payment_date, "+
			"nps_score = EXCLUDED.nps_score, "+
			"usage_score = EXCLUDED.usage_score, "+
			"churn_reason = EXCLUDED.churn_reason, "+
			"bot_active = EXCLUDED.bot_active, "+
			"blacklisted = EXCLUDED.blacklisted, "+
			"wa_undeliverable = EXCLUDED.wa_undeliverable, "+
			"response_status = EXCLUDED.response_status, "+
			"renewed = EXCLUDED.renewed, "+
			"rejected = EXCLUDED.rejected, "+
			"sequence_cs = EXCLUDED.sequence_cs, "+
			"cross_sell_rejected = EXCLUDED.cross_sell_rejected, "+
			"cross_sell_interested = EXCLUDED.cross_sell_interested, "+
			"cross_sell_resume_date = EXCLUDED.cross_sell_resume_date, "+
			"feature_update_sent = EXCLUDED.feature_update_sent, "+
			"days_since_cs_last_sent = EXCLUDED.days_since_cs_last_sent, "+
			"checkin_replied = EXCLUDED.checkin_replied, "+
			"pre14_sent = EXCLUDED.pre14_sent, "+
			"pre7_sent = EXCLUDED.pre7_sent, "+
			"pre3_sent = EXCLUDED.pre3_sent, "+
			"post1_sent = EXCLUDED.post1_sent, "+
			"post4_sent = EXCLUDED.post4_sent, "+
			"post8_sent = EXCLUDED.post8_sent, "+
			"post15_sent = EXCLUDED.post15_sent, "+
			"last_interaction_date = EXCLUDED.last_interaction_date, "+
			"workspace_id = EXCLUDED.workspace_id",
	)

	query, args, err := database.PSQL.
		Insert("clients").
		Columns(
			"company_id", "company_name", "pic_name", "pic_wa", "pic_email", "pic_role", "hc_size",
			"owner_name", "owner_wa", "owner_telegram_id",
			"segment", "plan_type", "payment_terms",
			"contract_months", "contract_start", "contract_end",
			"activation_date", "payment_status",
			"final_price", "quotation_link", "renewal_date", "notes",
			"last_payment_date",
			"nps_score", "usage_score",
			"churn_reason",
			"bot_active", "blacklisted", "wa_undeliverable",
			"renewed", "rejected",
			"sequence_cs",
			"cross_sell_rejected", "cross_sell_interested", "cross_sell_resume_date",
			"feature_update_sent", "days_since_cs_last_sent",
			"checkin_replied", "response_status",
			"pre14_sent", "pre7_sent", "pre3_sent",
			"post1_sent", "post4_sent", "post8_sent", "post15_sent",
			"last_interaction_date",
			"workspace_id",
		).
		Values(
			client.CompanyID, client.CompanyName, client.PICName, client.PICWA, client.PICEmail, client.PICRole, client.HCSize,
			client.OwnerName, client.OwnerWA, client.OwnerTelegramID,
			client.Segment, client.PlanType, client.PaymentTerms,
			client.ContractMonths, client.ContractStart, client.ContractEnd,
			client.ActivationDate, client.PaymentStatus,
			client.FinalPrice, client.QuotationLink, client.RenewalDate, client.Notes,
			client.LastPaymentDate,
			client.NPSScore, client.UsageScore,
			client.ChurnReason,
			client.BotActive, client.Blacklisted, client.WAUndeliverable,
			client.Renewed, client.Rejected,
			client.SequenceCS,
			client.CrossSellRejected, client.CrossSellInterested, client.CrossSellResumeDate,
			client.FeatureUpdateSent, client.DaysSinceCSLastSent,
			client.CheckinReplied, client.ResponseStatus,
			client.Pre14Sent, client.Pre7Sent, client.Pre3Sent,
			client.Post1Sent, client.Post4Sent, client.Post8Sent, client.Post15Sent,
			client.LastInteractionDate,
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

// UpdateLastInteraction sets the last_interaction_date for a client.
func (r *clientRepo) UpdateLastInteraction(ctx context.Context, companyID string, t time.Time) error {
	ctx, span := r.tracer.Start(ctx, "client.repository.UpdateLastInteraction")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Update("clients").
		Set("last_interaction_date", t).
		Where(sq.Eq{"company_id": companyID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query UpdateLastInteraction: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec UpdateLastInteraction: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", companyID)
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
		"SELECT %s FROM clients c WHERE c.blacklisted = false AND c.workspace_id = (SELECT id FROM workspaces WHERE slug = $1) ORDER BY c.company_id",
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
		"SELECT %s FROM clients c WHERE c.blacklisted = false AND c.workspace_id::text = ANY($1) ORDER BY c.company_id",
		clientColumns,
	)

	ids := make([]interface{}, len(workspaceIDs))
	for i, id := range workspaceIDs {
		ids[i] = id
	}

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

// UpdateClientFields dynamically updates specified fields for a client.
func (r *clientRepo) UpdateClientFields(ctx context.Context, companyID string, fields map[string]interface{}) error {
	ctx, span := r.tracer.Start(ctx, "client.repository.UpdateClientFields")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	if len(fields) == 0 {
		return nil
	}

	query, args, err := database.PSQL.
		Update("clients").
		SetMap(fields).
		Where(sq.Eq{"company_id": companyID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query UpdateClientFields: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec UpdateClientFields: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("client not found: %s", companyID)
	}
	return nil
}
