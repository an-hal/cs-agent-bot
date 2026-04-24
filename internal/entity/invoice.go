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
	// CompanyName is resolved from master_data at query-time and included on
	// the top-level Invoice shape so FE list views don't need a separate
	// fetch. Empty when the join misses (e.g. deleted client).
	CompanyName      string     `json:"company_name,omitempty"`
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

	// Optional: non-empty for multi-termin partial-payment invoices. Each
	// entry tracks one tranche — see Termin struct. Sum of entry amounts
	// should equal Invoice.Amount (validated by usecase).
	TerminBreakdown []Termin `json:"termin_breakdown,omitempty"`
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
// LunasPct is a [0, 100] float so FE can render as-is; UniqueCompanies lets
// FE show "N companies billed this period" alongside the total count.
type InvoiceStats struct {
	Total             int64            `json:"total"`
	TotalAmount       int64            `json:"total_amount"`
	UniqueCompanies   int64            `json:"unique_companies"`
	LunasPct          float64          `json:"lunas_pct"`
	ByStatus          map[string]int64 `json:"by_status"`
	AmountByStatus    map[string]int64 `json:"amount_by_status"`
	ByCollectionStage map[string]int64 `json:"by_collection_stage"`
}

// PaymentMethodRoute enums — FE spec requires BE to validate which rail the
// invoice will collect through.
const (
	PaymentMethodRoutePaperID  = "paper_id"       // hosted-checkout via Paper.id
	PaymentMethodRouteTransfer = "transfer_bank"  // manual reconciliation by AE
)

// CreateInvoiceReq carries everything needed to create a new invoice.
type CreateInvoiceReq struct {
	CompanyID           string            `json:"company_id"`
	IssueDate           time.Time         `json:"issue_date"`
	DueDate             time.Time         `json:"due_date"`
	PaymentTerms        int               `json:"payment_terms"`
	Notes               string            `json:"notes"`
	CreatedBy           string            `json:"created_by"`
	LineItems           []InvoiceLineItem `json:"line_items"`
	// PaymentMethodRoute selects the collection rail (paper_id | transfer_bank).
	// Empty defaults to transfer_bank (backwards-compatible).
	PaymentMethodRoute  string            `json:"payment_method_route"`
	// TerminBreakdown — when non-empty, marks the invoice as multi-termin.
	// Sum of amounts must equal total invoice amount (validated by usecase).
	TerminBreakdown     []Termin          `json:"termin_breakdown,omitempty"`
}

// IsValidPaymentMethodRoute reports whether s is an accepted payment route.
// Empty string is accepted (defaults to transfer_bank upstream).
func IsValidPaymentMethodRoute(s string) bool {
	switch s {
	case "", PaymentMethodRoutePaperID, PaymentMethodRouteTransfer:
		return true
	}
	return false
}

// MarkPaidReq carries the data needed to manually mark an invoice as paid.
type MarkPaidReq struct {
	PaymentMethod string    `json:"payment_method"`
	PaymentDate   time.Time `json:"payment_date"`
	Notes         string    `json:"notes"`
	AmountPaid    int64     `json:"amount_paid"`
	Actor         string    `json:"actor"`
}

// Termin is one tranche of a multi-part payment. Stored on an invoice as a
// slice under Invoice.TerminBreakdown (JSONB).
type Termin struct {
	TerminNumber  int       `json:"termin_number"`
	Amount        int64     `json:"amount"`
	DueDate       time.Time `json:"due_date"`
	Status        string    `json:"status"`           // pending|paid|overdue
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	PaymentMethod string    `json:"payment_method,omitempty"`
	PaymentRef    string    `json:"payment_ref,omitempty"`
	Notes         string    `json:"notes,omitempty"`
}

// ConfirmPartialReq is the payload for AE-only confirm-partial endpoint.
type ConfirmPartialReq struct {
	TerminNumber  int       `json:"termin_number"`
	AmountPaid    int64     `json:"amount_paid"`
	PaymentMethod string    `json:"payment_method"`
	PaymentRef    string    `json:"payment_ref"`
	PaidAt        time.Time `json:"paid_at"`
	Actor         string    `json:"-"`
	Notes         string    `json:"notes"`
}

// UpdateStageReq for POST /invoices/{id}/update-stage — manual collection-stage
// override (typically used when AE negotiates offline and needs to move a
// client to a softer/firmer stage than the cron would have assigned).
type UpdateStageReq struct {
	NewStage string `json:"new_stage"`
	Reason   string `json:"reason"`
	Actor    string `json:"-"`
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
