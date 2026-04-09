package repository

import (
	"context"
	"database/sql"
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

type InvoiceRepository interface {
	GetActiveByCompanyID(ctx context.Context, companyID string) (*entity.Invoice, error)
	GetAllByCompanyID(ctx context.Context, companyID string) ([]entity.Invoice, error)
	GetAllByCompanyIDPaginated(ctx context.Context, companyID string, p pagination.Params) ([]entity.Invoice, int64, error)
	GetAllPaginated(ctx context.Context, filter entity.InvoiceFilter, p pagination.Params) ([]entity.Invoice, int64, error)
	GetByID(ctx context.Context, invoiceID string) (*entity.Invoice, error)
	UpdateFields(ctx context.Context, invoiceID string, fields map[string]interface{}) error
	CreateInvoice(ctx context.Context, inv entity.Invoice) error
	UpdateFlags(ctx context.Context, invoiceID string, flags map[string]bool) error
}

type invoiceRepo struct {
	DB           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewInvoiceRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) InvoiceRepository {
	return &invoiceRepo{
		DB:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *invoiceRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *invoiceRepo) GetActiveByCompanyID(ctx context.Context, companyID string) (*entity.Invoice, error) {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.GetActiveByCompanyID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("invoice_id", "company_id", "issue_date", "due_date", "amount", "payment_status", "paid_at", "COALESCE(amount_paid, 0)", "reminder_count", "collection_stage", "created_at", "COALESCE(notes, '') as notes", "COALESCE(link_invoice, '') as link_invoice", "last_reminder_date", "COALESCE(workspace_id::text, '') as workspace_id").
		From("invoices").
		Where(sq.And{
			sq.Eq{"company_id": companyID},
			sq.NotEq{"payment_status": entity.PaymentStatusPaid},
		}).
		OrderBy("due_date DESC").
		Limit(1).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	var inv entity.Invoice
	err = r.DB.QueryRowContext(ctx, query, args...).Scan(
		&inv.InvoiceID,
		&inv.CompanyID,
		&inv.IssueDate,
		&inv.DueDate,
		&inv.Amount,
		&inv.PaymentStatus,
		&inv.PaidAt,
		&inv.AmountPaid,
		&inv.ReminderCount,
		&inv.CollectionStage,
		&inv.CreatedAt,
		&inv.Notes,
		&inv.LinkInvoice,
		&inv.LastReminderDate,
		&inv.WorkspaceID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query invoice: %w", err)
	}

	return &inv, nil
}

func (r *invoiceRepo) CreateInvoice(ctx context.Context, inv entity.Invoice) error {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.CreateInvoice")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Insert("invoices").
		Columns("invoice_id", "company_id", "due_date", "payment_status").
		Values(inv.InvoiceID, inv.CompanyID, inv.DueDate, inv.PaymentStatus).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err = r.DB.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("insert invoice: %w", err)
	}

	return nil
}

func (r *invoiceRepo) UpdateFlags(ctx context.Context, invoiceID string, flags map[string]bool) error {
	_, span := r.tracer.Start(ctx, "invoice.repository.UpdateFlags")
	defer span.End()

	r.logger.Warn().Str("invoice_id", invoiceID).Msg(
		"UpdateFlags called but Invoice table lacks reminder flag columns. " +
			"Consider adding columns: Pre14Sent, Pre7Sent, Pre3Sent, Post1Sent, Post4Sent, Post8Sent")
	return nil
}

// GetAllByCompanyIDPaginated returns paginated invoices for a given company, newest first.
func (r *invoiceRepo) GetAllByCompanyIDPaginated(ctx context.Context, companyID string, p pagination.Params) ([]entity.Invoice, int64, error) {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.GetAllByCompanyIDPaginated")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	// Count
	countQ, countArgs, err := database.PSQL.
		Select("COUNT(*)").
		From("invoices").
		Where(sq.Eq{"company_id": companyID}).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	var total int64
	if scanErr := r.DB.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); scanErr != nil {
		return nil, 0, fmt.Errorf("count invoices: %w", scanErr)
	}

	// Data
	dataQ, dataArgs, err := database.PSQL.
		Select("invoice_id", "company_id", "issue_date", "due_date", "amount", "payment_status", "paid_at", "COALESCE(amount_paid, 0)", "reminder_count", "collection_stage", "created_at", "COALESCE(notes, '') as notes", "COALESCE(link_invoice, '') as link_invoice", "last_reminder_date", "COALESCE(workspace_id::text, '') as workspace_id").
		From("invoices").
		Where(sq.Eq{"company_id": companyID}).
		OrderBy("created_at DESC").
		Limit(uint64(p.Limit)).
		Offset(uint64(p.Offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build data query: %w", err)
	}

	rows, err := r.DB.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query invoices: %w", err)
	}
	defer rows.Close()

	var invoices []entity.Invoice
	for rows.Next() {
		var inv entity.Invoice
		if err := rows.Scan(
			&inv.InvoiceID, &inv.CompanyID, &inv.IssueDate, &inv.DueDate, &inv.Amount, &inv.PaymentStatus,
			&inv.PaidAt, &inv.AmountPaid, &inv.ReminderCount, &inv.CollectionStage, &inv.CreatedAt,
			&inv.Notes, &inv.LinkInvoice, &inv.LastReminderDate, &inv.WorkspaceID,
		); err != nil {
			return nil, 0, fmt.Errorf("scan invoice: %w", err)
		}
		invoices = append(invoices, inv)
	}
	return invoices, total, rows.Err()
}

// GetAllPaginated returns paginated invoices with optional filters.
func (r *invoiceRepo) GetAllPaginated(ctx context.Context, filter entity.InvoiceFilter, p pagination.Params) ([]entity.Invoice, int64, error) {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.GetAllPaginated")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	where := sq.And{}
	if len(filter.WorkspaceIDs) > 0 {
		where = append(where, sq.Expr("workspace_id::text = ANY(?)", pq.Array(filter.WorkspaceIDs)))
	}
	if filter.CompanyID != "" {
		where = append(where, sq.Eq{"company_id": filter.CompanyID})
	}
	if filter.Status != "" {
		where = append(where, sq.Eq{"payment_status": filter.Status})
	}
	if filter.CollectionStage != "" {
		where = append(where, sq.Eq{"collection_stage": filter.CollectionStage})
	}
	if filter.Search != "" {
		pattern := "%" + filter.Search + "%"
		where = append(where, sq.Or{
			sq.ILike{"invoice_id": pattern},
			sq.ILike{"company_id": pattern},
			sq.ILike{"notes": pattern},
			sq.ILike{"collection_stage": pattern},
		})
	}

	// Count
	countBuilder := database.PSQL.Select("COUNT(*)").From("invoices")
	if len(where) > 0 {
		countBuilder = countBuilder.Where(where)
	}
	countQ, countArgs, err := countBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count query: %w", err)
	}
	var total int64
	if scanErr := r.DB.QueryRowContext(ctx, countQ, countArgs...).Scan(&total); scanErr != nil {
		return nil, 0, fmt.Errorf("count invoices: %w", scanErr)
	}

	// Data
	dataBuilder := database.PSQL.
		Select("invoice_id", "company_id", "issue_date", "due_date", "amount", "payment_status", "paid_at", "COALESCE(amount_paid, 0)", "reminder_count", "collection_stage", "created_at", "COALESCE(notes, '') as notes", "COALESCE(link_invoice, '') as link_invoice", "last_reminder_date", "COALESCE(workspace_id::text, '') as workspace_id").
		From("invoices").
		OrderBy("created_at DESC").
		Limit(uint64(p.Limit)).
		Offset(uint64(p.Offset))
	if len(where) > 0 {
		dataBuilder = dataBuilder.Where(where)
	}
	dataQ, dataArgs, err := dataBuilder.ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build data query: %w", err)
	}

	rows, err := r.DB.QueryContext(ctx, dataQ, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query invoices: %w", err)
	}
	defer rows.Close()

	var invoices []entity.Invoice
	for rows.Next() {
		var inv entity.Invoice
		if err := rows.Scan(
			&inv.InvoiceID, &inv.CompanyID, &inv.IssueDate, &inv.DueDate, &inv.Amount, &inv.PaymentStatus,
			&inv.PaidAt, &inv.AmountPaid, &inv.ReminderCount, &inv.CollectionStage, &inv.CreatedAt,
			&inv.Notes, &inv.LinkInvoice, &inv.LastReminderDate, &inv.WorkspaceID,
		); err != nil {
			return nil, 0, fmt.Errorf("scan invoice: %w", err)
		}
		invoices = append(invoices, inv)
	}
	return invoices, total, rows.Err()
}

// GetByID returns a single invoice by its ID.
func (r *invoiceRepo) GetByID(ctx context.Context, invoiceID string) (*entity.Invoice, error) {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.GetByID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("invoice_id", "company_id", "issue_date", "due_date", "amount", "payment_status", "paid_at", "COALESCE(amount_paid, 0)", "reminder_count", "collection_stage", "created_at", "COALESCE(notes, '') as notes", "COALESCE(link_invoice, '') as link_invoice", "last_reminder_date", "COALESCE(workspace_id::text, '') as workspace_id").
		From("invoices").
		Where(sq.Eq{"invoice_id": invoiceID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	var inv entity.Invoice
	err = r.DB.QueryRowContext(ctx, query, args...).Scan(
		&inv.InvoiceID, &inv.CompanyID, &inv.IssueDate, &inv.DueDate, &inv.Amount, &inv.PaymentStatus,
		&inv.PaidAt, &inv.AmountPaid, &inv.ReminderCount, &inv.CollectionStage, &inv.CreatedAt,
		&inv.Notes, &inv.LinkInvoice, &inv.LastReminderDate, &inv.WorkspaceID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query invoice: %w", err)
	}
	return &inv, nil
}

// UpdateFields updates specific fields on an invoice.
func (r *invoiceRepo) UpdateFields(ctx context.Context, invoiceID string, fields map[string]interface{}) error {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.UpdateFields")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Update("invoices").
		SetMap(fields).
		Where(sq.Eq{"invoice_id": invoiceID}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err = r.DB.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update invoice: %w", err)
	}
	return nil
}

// GetAllByCompanyID returns all invoices for a given company, newest first.
func (r *invoiceRepo) GetAllByCompanyID(ctx context.Context, companyID string) ([]entity.Invoice, error) {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.GetAllByCompanyID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("invoice_id", "company_id", "issue_date", "due_date", "amount", "payment_status", "paid_at", "COALESCE(amount_paid, 0)", "reminder_count", "collection_stage", "created_at", "COALESCE(notes, '') as notes", "COALESCE(link_invoice, '') as link_invoice", "last_reminder_date", "COALESCE(workspace_id::text, '') as workspace_id").
		From("invoices").
		Where(sq.Eq{"company_id": companyID}).
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query invoices: %w", err)
	}
	defer rows.Close()

	var invoices []entity.Invoice
	for rows.Next() {
		var inv entity.Invoice
		if err := rows.Scan(
			&inv.InvoiceID, &inv.CompanyID, &inv.IssueDate, &inv.DueDate, &inv.Amount, &inv.PaymentStatus,
			&inv.PaidAt, &inv.AmountPaid, &inv.ReminderCount, &inv.CollectionStage, &inv.CreatedAt,
			&inv.Notes, &inv.LinkInvoice, &inv.LastReminderDate, &inv.WorkspaceID,
		); err != nil {
			return nil, fmt.Errorf("scan invoice: %w", err)
		}
		invoices = append(invoices, inv)
	}
	return invoices, rows.Err()
}
