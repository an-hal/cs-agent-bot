package entity

import (
	"time"
)

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
}

// InvoiceFilter holds optional filters for listing invoices.
type InvoiceFilter struct {
	WorkspaceIDs    []string // one or more workspace UUIDs (holding workspaces expand to member IDs)
	CompanyID       string
	Status          string // payment_status exact match
	Search          string // ILIKE across invoice_id, company_id, notes, collection_stage
	CollectionStage string // exact match
}

// DaysPastDue returns how many days past the due date. Negative means not yet due.
func (inv *Invoice) DaysPastDue() int {
	return int(time.Since(inv.DueDate).Hours() / 24)
}

// DaysUntilDue returns how many days until the due date. Negative means past due.
func (inv *Invoice) DaysUntilDue() int {
	return int(time.Until(inv.DueDate).Hours() / 24)
}
