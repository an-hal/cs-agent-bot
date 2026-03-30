package repository

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database/sheets"
	"github.com/Sejutacita/cs-agent-bot/internal/service/cache"
	"github.com/rs/zerolog"
)

type InvoiceRepository interface {
	GetActiveByCompanyID(ctx context.Context, companyID string) (*entity.Invoice, error)
	CreateInvoice(ctx context.Context, inv entity.Invoice) error
	UpdateFlags(ctx context.Context, invoiceID string, flags map[string]bool) error
}

type invoiceRepo struct {
	sheetsClient *sheets.SheetsClient
	cache        cache.SheetCache
	logger       zerolog.Logger
}

func NewInvoiceRepo(sheetsClient *sheets.SheetsClient, cache cache.SheetCache, logger zerolog.Logger) InvoiceRepository {
	return &invoiceRepo{
		sheetsClient: sheetsClient,
		cache:        cache,
		logger:       logger,
	}
}

func (r *invoiceRepo) GetActiveByCompanyID(ctx context.Context, companyID string) (*entity.Invoice, error) {
	rows, err := r.cache.Get(ctx, cache.KeyInvoices, cache.RangeInvoices, cache.TTLInvoices)
	if err != nil {
		return nil, err
	}

	// Find the latest invoice for this company that is not Paid
	var latest *entity.Invoice
	// Skip first 3 rows (indices 0-2): 2 info rows + 1 column header row
	// Data starts at row 4 (index 3)
	for i, row := range rows {
		if i < 3 {
			continue // skip header rows
		}
		inv, err := parseInvoiceRow(row)
		if err != nil {
			continue
		}
		if inv.CompanyID == companyID && inv.PaymentStatus != entity.PaymentStatusPaid {
			latest = inv
		}
	}

	if latest == nil {
		return nil, nil // no active invoice, not an error
	}

	return latest, nil
}

func (r *invoiceRepo) CreateInvoice(ctx context.Context, inv entity.Invoice) error {
	row := invoiceToRow(inv)
	if err := r.sheetsClient.AppendRows(ctx, cache.RangeInvoices, [][]interface{}{row}); err != nil {
		return err
	}
	return r.cache.Invalidate(ctx, cache.KeyInvoices)
}

func (r *invoiceRepo) UpdateFlags(ctx context.Context, invoiceID string, flags map[string]bool) error {
	// Note: The Invoice sheet doesn't have Pre14Sent, Pre7Sent, Pre3Sent, Post1Sent, Post4Sent, Post8Sent columns.
	// These flags need to be tracked separately or columns added to the sheet.
	r.logger.Warn().Str("invoice_id", invoiceID).Msg(
		"UpdateFlags called but Invoice sheet lacks reminder flag columns. " +
			"Consider adding columns: Pre14Sent, Pre7Sent, Pre3Sent, Post1Sent, Post4Sent, Post8Sent")
	return nil
}

// parseInvoiceRow maps a Sheets row to entity.Invoice.
// Column order based on "POC Project Bumi – AE HRIS Automation.xlsx" Sheet 2: Invoice & Billing
// Row 3 contains the actual column names.
func parseInvoiceRow(row []interface{}) (*entity.Invoice, error) {
	if len(row) < 7 {
		return nil, fmt.Errorf("invoice row too short: %d columns, expected at least 7", len(row))
	}

	return &entity.Invoice{
		InvoiceID:     safeString(row, 0), // Invoice_ID
		CompanyID:     safeString(row, 1), // Company_ID
		DueDate:       safeDate(row, 5),   // Due_Date
		Amount:        safeFloat(row, 3),  // Amount (Rp)
		PaymentStatus: safeString(row, 6), // Payment_Status
		// Flags not in sheet: Pre14Sent, Pre7Sent, Pre3Sent, Post1Sent, Post4Sent, Post8Sent
		// These remain false as they're not tracked in the current sheet design
	}, nil
}

// invoiceToRow converts entity.Invoice to a Sheets row.
// Must match parseInvoiceRow column order.
func invoiceToRow(inv entity.Invoice) []interface{} {
	// Create a 12-column row (0-11) with empty values for unmapped columns
	row := make([]interface{}, 12)

	row[0] = inv.InvoiceID
	row[1] = inv.CompanyID
	// Col 2: Company_Name [lookup]
	row[3] = inv.Amount
	// Col 4: Issue_Date (not in entity)
	row[5] = inv.DueDate.Format("2006-01-02")
	row[6] = inv.PaymentStatus
	// Cols 7-11: Days_Overdue [computed], Reminder_Count, Last_Reminder, Collection_Stage, Notes

	return row
}

// columnLetter converts a 0-based column index to a spreadsheet column letter.
func columnLetter(idx int) string {
	result := ""
	for idx >= 0 {
		result = string(rune('A'+idx%26)) + result
		idx = idx/26 - 1
	}
	return result
}
