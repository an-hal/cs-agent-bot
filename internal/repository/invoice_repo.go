package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type InvoiceRepository interface {
	GetActiveByCompanyID(ctx context.Context, companyID string) (*entity.Invoice, error)
	GetAllByCompanyID(ctx context.Context, companyID string) ([]entity.Invoice, error)
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
		Select("invoice_id", "company_id", "issue_date", "due_date", "amount", "payment_status", "paid_at", "amount_paid", "reminder_count", "collection_stage", "created_at", "COALESCE(notes, '') as notes", "COALESCE(link_invoice, '') as link_invoice", "last_reminder_date", "COALESCE(workspace_id::text, '') as workspace_id").
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
	ctx, span := r.tracer.Start(ctx, "invoice.repository.UpdateFlags")
	defer span.End()

	r.logger.Warn().Str("invoice_id", invoiceID).Msg(
		"UpdateFlags called but Invoice table lacks reminder flag columns. " +
			"Consider adding columns: Pre14Sent, Pre7Sent, Pre3Sent, Post1Sent, Post4Sent, Post8Sent")
	return nil
}

// GetAllByCompanyID returns all invoices for a given company, newest first.
func (r *invoiceRepo) GetAllByCompanyID(ctx context.Context, companyID string) ([]entity.Invoice, error) {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.GetAllByCompanyID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("invoice_id", "company_id", "issue_date", "due_date", "amount", "payment_status", "paid_at", "amount_paid", "reminder_count", "collection_stage", "created_at", "COALESCE(notes, '') as notes", "COALESCE(link_invoice, '') as link_invoice", "last_reminder_date", "COALESCE(workspace_id::text, '') as workspace_id").
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
