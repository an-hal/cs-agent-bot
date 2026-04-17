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

// InvoiceLineItemRepository handles persistence for invoice line items.
type InvoiceLineItemRepository interface {
	BulkCreate(ctx context.Context, tx *sql.Tx, items []entity.InvoiceLineItem) error
	GetByInvoiceID(ctx context.Context, invoiceID string) ([]entity.InvoiceLineItem, error)
	DeleteByInvoiceID(ctx context.Context, tx *sql.Tx, invoiceID string) error
}

type invoiceLineItemRepo struct {
	DB           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewInvoiceLineItemRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) InvoiceLineItemRepository {
	return &invoiceLineItemRepo{
		DB:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *invoiceLineItemRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// BulkCreate inserts multiple line items for an invoice, within an optional transaction.
func (r *invoiceLineItemRepo) BulkCreate(ctx context.Context, tx *sql.Tx, items []entity.InvoiceLineItem) error {
	ctx, span := r.tracer.Start(ctx, "invoice_line_item.repository.BulkCreate")
	defer span.End()

	if len(items) == 0 {
		return nil
	}

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	builder := database.PSQL.
		Insert("invoice_line_items").
		Columns("invoice_id", "workspace_id", "description", "qty", "unit_price", "subtotal", "sort_order")

	for _, item := range items {
		builder = builder.Values(
			item.InvoiceID, item.WorkspaceID, item.Description,
			item.Qty, item.UnitPrice, item.Subtotal, item.SortOrder,
		)
	}

	query, args, err := builder.ToSql()
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
		return fmt.Errorf("bulk insert line items: %w", err)
	}
	return nil
}

// GetByInvoiceID returns all line items for a given invoice, ordered by sort_order.
func (r *invoiceLineItemRepo) GetByInvoiceID(ctx context.Context, invoiceID string) ([]entity.InvoiceLineItem, error) {
	ctx, span := r.tracer.Start(ctx, "invoice_line_item.repository.GetByInvoiceID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select("id::text", "invoice_id", "workspace_id::text", "description", "qty", "unit_price", "subtotal", "sort_order", "created_at").
		From("invoice_line_items").
		Where(sq.Eq{"invoice_id": invoiceID}).
		OrderBy("sort_order ASC").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query line items: %w", err)
	}
	defer rows.Close()

	var items []entity.InvoiceLineItem
	for rows.Next() {
		var item entity.InvoiceLineItem
		if err := rows.Scan(
			&item.ID, &item.InvoiceID, &item.WorkspaceID,
			&item.Description, &item.Qty, &item.UnitPrice, &item.Subtotal,
			&item.SortOrder, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan line item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeleteByInvoiceID removes all line items for an invoice, within an optional transaction.
func (r *invoiceLineItemRepo) DeleteByInvoiceID(ctx context.Context, tx *sql.Tx, invoiceID string) error {
	ctx, span := r.tracer.Start(ctx, "invoice_line_item.repository.DeleteByInvoiceID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Delete("invoice_line_items").
		Where(sq.Eq{"invoice_id": invoiceID}).
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
		return fmt.Errorf("delete line items: %w", err)
	}
	return nil
}
