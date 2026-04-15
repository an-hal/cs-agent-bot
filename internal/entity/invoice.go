package entity

import (
	"time"
)

// Payment status constants (Indonesian names, as stored in DB).
const (
	PaymentStatusLunas     = "Lunas"
	PaymentStatusMenunggu  = "Menunggu"
	PaymentStatusBelumBayar = "Belum bayar"
	PaymentStatusTerlambat = "Terlambat"
)

// Collection stage constants.
const (
	CollectionStage0 = "Stage 0 — Pre-due"
	CollectionStage1 = "Stage 1 — Soft"
	CollectionStage2 = "Stage 2 — Firm"
	CollectionStage3 = "Stage 3 — Urgency"
	CollectionStage4 = "Stage 4 — Escalate"
	CollectionStageClosed = "Closed"
)

// EventType constants for payment_logs.
const (
	EventPaymentReceived  = "payment_received"
	EventManualMarkPaid   = "manual_mark_paid"
	EventReminderSent     = "reminder_sent"
	EventStageChange      = "stage_change"
	EventStatusChange     = "status_change"
	EventPaperIDWebhook   = "paperid_webhook"
	EventInvoiceCreated   = "invoice_created"
	EventInvoiceUpdated   = "invoice_updated"
)

// Invoice is the core billing document.
// NOTE: payment_status must only be written by the Paper.id webhook handler
// or the cron overdue-status updater. Application code must not set it directly.
type Invoice struct {
	InvoiceID        string     `json:"invoice_id"`
	CompanyID        string     `json:"company_id"`
	IssueDate        time.Time  `json:"issue_date"`
	DueDate          time.Time  `json:"due_date"`
	Amount           float64    `json:"amount"`
	PaymentStatus    string     `json:"payment_status"`
	PaidAt           *time.Time `json:"paid_at"`
	AmountPaid       float64    `json:"amount_paid"`
	ReminderCount    int        `json:"reminder_count"`
	CollectionStage  string     `json:"collection_stage"`
	CreatedAt        time.Time  `json:"created_at"`
	Notes            string     `json:"notes"`
	LinkInvoice      string     `json:"link_invoice"`
	LastReminderDate *time.Time `json:"last_reminder_date"`
	WorkspaceID      string     `json:"workspace_id"`

	// Fields added in migration 000609.
	PaymentTerms  int        `json:"payment_terms"`
	PaymentMethod string     `json:"payment_method"`
	PaymentDate   *time.Time `json:"payment_date"`
	DaysOverdue   int        `json:"days_overdue"`
	PaperIDURL    string     `json:"paper_id_url"`
	PaperIDRef    string     `json:"paper_id_ref"`
	CreatedBy     string     `json:"created_by"`
	UpdatedAt     *time.Time `json:"updated_at"`
}

// InvoiceFilter holds optional filters for listing invoices.
type InvoiceFilter struct {
	WorkspaceIDs    []string // one or more workspace UUIDs (holding workspaces expand to member IDs)
	CompanyID       string
	Status          string // payment_status exact match
	Search          string // ILIKE across invoice_id, company_id, notes, collection_stage
	CollectionStage string // exact match
	SortBy          string // column name to sort by
	SortDir         string // "asc" or "desc"
}

// InvoiceLineItem is a single line on an invoice.
type InvoiceLineItem struct {
	ID          string    `json:"id"`
	InvoiceID   string    `json:"invoice_id"`
	WorkspaceID string    `json:"workspace_id"`
	Description string    `json:"description"`
	Qty         int       `json:"qty"`
	UnitPrice   int64     `json:"unit_price"`
	Subtotal    int64     `json:"subtotal"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
}

// PaymentLog is an append-only audit record for every payment event.
type PaymentLog struct {
	ID             string         `json:"id"`
	WorkspaceID    string         `json:"workspace_id"`
	InvoiceID      string         `json:"invoice_id"`
	EventType      string         `json:"event_type"`
	AmountPaid     *int64         `json:"amount_paid,omitempty"`
	PaymentMethod  string         `json:"payment_method"`
	PaymentChannel string         `json:"payment_channel"`
	PaymentRef     string         `json:"payment_ref"`
	OldStatus      string         `json:"old_status"`
	NewStatus      string         `json:"new_status"`
	OldStage       string         `json:"old_stage"`
	NewStage       string         `json:"new_stage"`
	Actor          string         `json:"actor"`
	Notes          string         `json:"notes"`
	RawPayload     map[string]any `json:"raw_payload,omitempty"`
	Timestamp      time.Time      `json:"timestamp"`
}

// InvoiceDetail is a full invoice with line items, payment logs, and client name.
type InvoiceDetail struct {
	Invoice
	CompanyName string            `json:"company_name"`
	LineItems   []InvoiceLineItem `json:"line_items"`
	PaymentLogs []PaymentLog      `json:"payment_logs"`
}

// InvoiceStats holds aggregated stat-card data for the invoice dashboard.
type InvoiceStats struct {
	Total             int64            `json:"total"`
	TotalAmount       int64            `json:"total_amount"`
	ByStatus          map[string]int64 `json:"by_status"`
	AmountByStatus    map[string]int64 `json:"amount_by_status"`
	ByCollectionStage map[string]int64 `json:"by_collection_stage"`
}

// CreateInvoiceReq carries everything needed to create a new invoice.
type CreateInvoiceReq struct {
	CompanyID    string            `json:"company_id"`
	IssueDate    time.Time         `json:"issue_date"`
	DueDate      time.Time         `json:"due_date"`
	PaymentTerms int               `json:"payment_terms"`
	Notes        string            `json:"notes"`
	CreatedBy    string            `json:"created_by"`
	LineItems    []InvoiceLineItem `json:"line_items"`
}

// MarkPaidReq carries the data needed to manually mark an invoice as paid.
type MarkPaidReq struct {
	PaymentMethod string    `json:"payment_method"`
	PaymentDate   time.Time `json:"payment_date"`
	Notes         string    `json:"notes"`
	AmountPaid    int64     `json:"amount_paid"`
	Actor         string    `json:"actor"`
}

// SendReminderReq carries the parameters for dispatching a payment reminder.
type SendReminderReq struct {
	Channel    string `json:"channel"`
	TemplateID string `json:"template_id"`
	Actor      string `json:"actor"`
}

// DaysPastDue returns how many days past the due date. Negative means not yet due.
func (inv *Invoice) DaysPastDue() int {
	return int(time.Since(inv.DueDate).Hours() / 24)
}

// DaysUntilDue returns how many days until the due date. Negative means past due.
func (inv *Invoice) DaysUntilDue() int {
	return int(time.Until(inv.DueDate).Hours() / 24)
}
