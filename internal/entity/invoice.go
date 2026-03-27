package entity

import (
	"time"
)

type Invoice struct {
	InvoiceID     string    `json:"invoice_id"`
	CompanyID     string    `json:"company_id"`
	DueDate       time.Time `json:"due_date"`
	Amount        float64   `json:"amount"`
	PaymentStatus string    `json:"payment_status"`
}

// DaysPastDue returns how many days past the due date. Negative means not yet due.
func (inv *Invoice) DaysPastDue() int {
	return int(time.Since(inv.DueDate).Hours() / 24)
}

// DaysUntilDue returns how many days until the due date. Negative means past due.
func (inv *Invoice) DaysUntilDue() int {
	return int(time.Until(inv.DueDate).Hours() / 24)
}
