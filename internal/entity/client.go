package entity

import (
	"fmt"
	"time"
)

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

	// Invoice reminder flags (from Master Client sheet columns 27-33)
	Pre14Sent  bool `json:"pre14_sent"`
	Pre7Sent   bool `json:"pre7_sent"`
	Pre3Sent   bool `json:"pre3_sent"`
	Post1Sent  bool `json:"post1_sent"`
	Post4Sent  bool `json:"post4_sent"`
	Post8Sent  bool `json:"post8_sent"`
	Post15Sent bool `json:"post15_sent"`
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

// DaysPastDue returns days since ContractEnd (payment overdue).
// Returns 0 if not yet past due.
func (c *Client) DaysPastDue() int {
	fmt.Println("Contract End: ", c.ContractEnd)
	fmt.Println("Days to Expiry: ", int(time.Until(c.ContractEnd).Hours()/24))
	fmt.Println("Current Time: ", time.Now())
	fmt.Println("Company Name :", c.CompanyName)
	fmt.Println("Company ID :", c.CompanyID)
	if c.ContractEnd.IsZero() {
		return 0
	}
	return int(time.Since(c.ContractEnd).Hours() / 24)
}

// IsPaymentOverdue checks if payment is overdue based on:
// - today > ContractEnd AND PaymentStatus != 'Paid'
func (c *Client) IsPaymentOverdue() bool {
	daysPast := c.DaysPastDue()
	isPaid := c.PaymentStatus == PaymentStatusPaid
	return daysPast > 0 && !isPaid
}

// HasPendingPayment checks if payment is pending but not yet overdue
func (c *Client) HasPendingPayment() bool {
	daysPast := c.DaysPastDue()
	isPaid := c.PaymentStatus == PaymentStatusPaid
	return daysPast <= 0 && !isPaid
}

// UpdatePaymentStatus updates the payment status and last interaction date
func (c *Client) UpdatePaymentStatus(status string) {
	c.PaymentStatus = status
	c.LastInteractionDate = time.Now()
}
