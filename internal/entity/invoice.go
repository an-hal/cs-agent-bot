package entity

import "time"

type Invoice struct {
	InvoiceID     string    `json:"invoice_id"`
	CompanyID     string    `json:"company_id"`
	DueDate       time.Time `json:"due_date"`
	Amount        float64   `json:"amount"`
	PaymentStatus string    `json:"payment_status"`
	Pre14Sent     bool      `json:"pre14_sent"`
	Pre7Sent      bool      `json:"pre7_sent"`
	Pre3Sent      bool      `json:"pre3_sent"`
	Post1Sent     bool      `json:"post1_sent"`
	Post4Sent     bool      `json:"post4_sent"`
	Post8Sent     bool      `json:"post8_sent"`
}

// DaysPastDue returns how many days past the due date. Negative means not yet due.
func (inv *Invoice) DaysPastDue() int {
	return int(time.Since(inv.DueDate).Hours() / 24)
}

// DaysUntilDue returns how many days until the due date. Negative means past due.
func (inv *Invoice) DaysUntilDue() int {
	return int(time.Until(inv.DueDate).Hours() / 24)
}
