package entity

import "time"

type Client struct {
	CompanyID           string    `json:"company_id"`
	CompanyName         string    `json:"company_name"`
	PICName             string    `json:"pic_name"`
	PICWA               string    `json:"pic_wa"`
	OwnerName           string    `json:"owner_name"`
	OwnerWA             string    `json:"owner_wa"`
	Segment             string    `json:"segment"`
	ContractMonths      int       `json:"contract_months"`
	ContractStart       time.Time `json:"contract_start"`
	ContractEnd         time.Time `json:"contract_end"`
	ActivationDate      time.Time `json:"activation_date"`
	PaymentStatus       string    `json:"payment_status"`
	NPSScore            int       `json:"nps_score"`
	UsageScore          int       `json:"usage_score"`
	BotActive           bool      `json:"bot_active"`
	Blacklisted         bool      `json:"blacklisted"`
	Renewed             bool      `json:"renewed"`
	Rejected            bool      `json:"rejected"`
	QuotationLink       string    `json:"quotation_link"`
	OwnerTelegramID     string    `json:"owner_telegram_id"`
	SequenceCS          string    `json:"sequence_cs"`
	CrossSellRejected   bool      `json:"cross_sell_rejected"`
	CrossSellInterested bool      `json:"cross_sell_interested"`
	CheckinReplied      bool      `json:"checkin_replied"`
	ResponseStatus      string    `json:"response_status"`
	LastInteractionDate time.Time `json:"last_interaction_date"`
}

// Segment constants
const (
	SegmentHigh = "High"
	SegmentMid  = "Mid"
	SegmentLow  = "Low"
)

// PaymentStatus constants
const (
	PaymentStatusPaid    = "Paid"
	PaymentStatusPending = "Pending"
	PaymentStatusOverdue = "Overdue"
	PaymentStatusPartial = "Partial"
)

// SequenceCS constants
const (
	SequenceCSActive    = "ACTIVE"
	SequenceCSLongterm  = "LONGTERM"
	SequenceCSSnoozed   = "SNOOZED"
	SequenceCSRejected  = "REJECTED"
	SequenceCSConverted = "CONVERTED"
)

// ResponseStatus constants
const (
	ResponseStatusPending = "Pending"
	ResponseStatusReplied = "Replied"
)

// DaysToExpiry returns the number of days until contract expiry.
func (c *Client) DaysToExpiry() int {
	return int(time.Until(c.ContractEnd).Hours() / 24)
}

// DaysSinceActivation returns the number of days since activation.
func (c *Client) DaysSinceActivation() int {
	return int(time.Since(c.ActivationDate).Hours() / 24)
}
