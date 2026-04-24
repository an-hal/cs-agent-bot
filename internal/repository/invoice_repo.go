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
	GetAllPaginated(ctx context.Context, filter entity.InvoiceFilter, p pagination.Params) ([]entity.Invoice, int64, error)
	GetByID(ctx context.Context, invoiceID string) (*entity.Invoice, error)
	UpdateFields(ctx context.Context, invoiceID string, fields map[string]interface{}) error
	CreateInvoice(ctx context.Context, inv entity.Invoice) error
	UpdateFlags(ctx context.Context, invoiceID string, flags map[string]bool) error

	// Full-featured methods used by the invoice usecase package.
	Create(ctx context.Context, tx *sql.Tx, inv entity.Invoice) error
	Delete(ctx context.Context, invoiceID string) error
	ListOverdue(ctx context.Context, cutoff time.Time) ([]entity.Invoice, error)
	Stats(ctx context.Context, wsIDs []string) (*entity.InvoiceStats, error)
	UpdateStatusBulk(ctx context.Context, invoiceIDs []string, newStatus string) error
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
			sq.NotEq{"payment_status": entity.PaymentStatusLunas},
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

// Create inserts a full invoice record within an optional transaction.
func (r *invoiceRepo) Create(ctx context.Context, tx *sql.Tx, inv entity.Invoice) error {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.Create")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Insert("invoices").
		Columns(
			"invoice_id", "company_id", "issue_date", "due_date", "amount",
			"payment_status", "collection_stage", "notes", "link_invoice",
			"workspace_id", "payment_terms", "created_by", "updated_at",
		).
		Values(
			inv.InvoiceID, inv.CompanyID, inv.IssueDate, inv.DueDate, inv.Amount,
			inv.PaymentStatus, inv.CollectionStage, inv.Notes, inv.LinkInvoice,
			inv.WorkspaceID, inv.PaymentTerms, inv.CreatedBy, time.Now().UTC(),
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	execFunc := func(q string, a ...interface{}) (sql.Result, error) {
		if tx != nil {
			return tx.ExecContext(ctx, q, a...)
		}
		return r.DB.ExecContext(ctx, q, a...)
	}

	if _, err = execFunc(query, args...); err != nil {
		return fmt.Errorf("insert invoice: %w", err)
	}
	return nil
}

// Delete removes an invoice. Only invoices with status 'Belum bayar' may be deleted.
func (r *invoiceRepo) Delete(ctx context.Context, invoiceID string) error {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.Delete")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Delete("invoices").
		Where(sq.And{
			sq.Eq{"invoice_id": invoiceID},
			sq.Eq{"payment_status": entity.PaymentStatusBelumBayar},
		}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	res, err := r.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete invoice: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("invoice not found or status is not %q", entity.PaymentStatusBelumBayar)
	}
	return nil
}

// ListOverdue returns all invoices that are past due_date and not yet paid/overdue.
func (r *invoiceRepo) ListOverdue(ctx context.Context, cutoff time.Time) ([]entity.Invoice, error) {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.ListOverdue")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(
			"invoice_id", "company_id", "workspace_id", "due_date",
			"payment_status", "collection_stage", "days_overdue",
		).
		From("invoices").
		Where(sq.And{
			sq.LtOrEq{"due_date": cutoff},
			sq.Expr("payment_status NOT IN (?, ?)", entity.PaymentStatusLunas, entity.PaymentStatusTerlambat),
		}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query overdue: %w", err)
	}
	defer rows.Close()

	var invoices []entity.Invoice
	for rows.Next() {
		var inv entity.Invoice
		if err := rows.Scan(
			&inv.InvoiceID, &inv.CompanyID, &inv.WorkspaceID, &inv.DueDate,
			&inv.PaymentStatus, &inv.CollectionStage, &inv.DaysOverdue,
		); err != nil {
			return nil, fmt.Errorf("scan overdue invoice: %w", err)
		}
		invoices = append(invoices, inv)
	}
	return invoices, rows.Err()
}

// Stats returns aggregated invoice statistics for a set of workspaces.
func (r *invoiceRepo) Stats(ctx context.Context, wsIDs []string) (*entity.InvoiceStats, error) {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.Stats")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	stats := &entity.InvoiceStats{
		ByStatus:          make(map[string]int64),
		AmountByStatus:    make(map[string]int64),
		ByCollectionStage: make(map[string]int64),
	}

	// Overall totals.
	_ = r.DB.QueryRowContext(ctx,
		"SELECT COUNT(*), COALESCE(SUM(amount::bigint),0) FROM invoices WHERE workspace_id::text = ANY($1)",
		pq.Array(wsIDs),
	).Scan(&stats.Total, &stats.TotalAmount)

	// Per payment_status.
	statusRows, err := r.DB.QueryContext(ctx,
		"SELECT payment_status, COUNT(*), COALESCE(SUM(amount::bigint),0) FROM invoices WHERE workspace_id::text = ANY($1) GROUP BY payment_status",
		pq.Array(wsIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("stats by status: %w", err)
	}
	defer statusRows.Close()
	for statusRows.Next() {
		var status string
		var cnt, amt int64
		if err := statusRows.Scan(&status, &cnt, &amt); err != nil {
			return nil, fmt.Errorf("scan status row: %w", err)
		}
		stats.ByStatus[status] = cnt
		stats.AmountByStatus[status] = amt
	}
	if err := statusRows.Err(); err != nil {
		return nil, err
	}

	// Per collection_stage.
	stageRows, err := r.DB.QueryContext(ctx,
		"SELECT COALESCE(collection_stage,'') AS stage, COUNT(*) FROM invoices WHERE workspace_id::text = ANY($1) GROUP BY stage",
		pq.Array(wsIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("stats by stage: %w", err)
	}
	defer stageRows.Close()
	for stageRows.Next() {
		var stage string
		var cnt int64
		if err := stageRows.Scan(&stage, &cnt); err != nil {
			return nil, fmt.Errorf("scan stage row: %w", err)
		}
		stats.ByCollectionStage[stage] = cnt
	}
	if err := stageRows.Err(); err != nil {
		return nil, err
	}

	// Unique companies billed across this scope — small scalar query, batched
	// after the aggregates above.
	_ = r.DB.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT company_id) FROM invoices WHERE workspace_id::text = ANY($1)",
		pq.Array(wsIDs),
	).Scan(&stats.UniqueCompanies)

	// Derive Lunas percentage from what we already have. Keeps the math in
	// one place so FE doesn't need to recompute.
	if stats.Total > 0 {
		if lunas, ok := stats.ByStatus[entity.PaymentStatusLunas]; ok {
			stats.LunasPct = (float64(lunas) / float64(stats.Total)) * 100.0
		}
	}
	return stats, nil
}

// UpdateStatusBulk sets payment_status for a list of invoices in one statement.
func (r *invoiceRepo) UpdateStatusBulk(ctx context.Context, invoiceIDs []string, newStatus string) error {
	ctx, span := r.tracer.Start(ctx, "invoice.repository.UpdateStatusBulk")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Update("invoices").
		Set("payment_status", newStatus).
		Set("updated_at", time.Now().UTC()).
		Where(sq.Expr("invoice_id = ANY(?)", pq.Array(invoiceIDs))).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err = r.DB.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("bulk update status: %w", err)
	}
	return nil
}
