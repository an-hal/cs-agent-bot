package entity

import (
	"time"
)

type Client struct {
	CompanyID           string     `json:"company_id"`
	CompanyName         string     `json:"company_name"`
	PICName             string     `json:"pic_name"`
	PICWA               string     `json:"pic_wa"`
	OwnerName           string     `json:"owner_name"`
	OwnerWA             *string    `json:"owner_wa"`
	Segment             string     `json:"segment"`
	ContractMonths      int        `json:"contract_months"`
	ContractStart       time.Time  `json:"contract_start"`
	ContractEnd         time.Time  `json:"contract_end"`
	ActivationDate      time.Time  `json:"activation_date"`
	PaymentStatus       string     `json:"payment_status"`
	NPSScore            int        `json:"nps_score"`
	UsageScore          int        `json:"usage_score"`
	BotActive           bool       `json:"bot_active"`
	Blacklisted         bool       `json:"blacklisted"`
	Renewed             bool       `json:"renewed"`
	Rejected            bool       `json:"rejected"`
	QuotationLink       string     `json:"quotation_link"`
	OwnerTelegramID     string     `json:"owner_telegram_id"`
	SequenceCS          string     `json:"sequence_cs"`
	CrossSellRejected   bool       `json:"cross_sell_rejected"`
	CrossSellInterested bool       `json:"cross_sell_interested"`
	CheckinReplied      bool       `json:"checkin_replied"`
	ResponseStatus      string     `json:"response_status"`
	LastInteractionDate *time.Time `json:"last_interaction_date"`

	// Invoice reminder flags (from Master Client sheet columns 27-33)
	Pre14Sent  bool `json:"pre14_sent"`
	Pre7Sent   bool `json:"pre7_sent"`
	Pre3Sent   bool `json:"pre3_sent"`
	Post1Sent  bool `json:"post1_sent"`
	Post4Sent  bool `json:"post4_sent"`
	Post8Sent  bool `json:"post8_sent"`
	Post15Sent bool `json:"post15_sent"`

	// Fields from existing DB columns previously not mapped
	PICEmail              string     `json:"pic_email"`
	PICRole               string     `json:"pic_role"`
	HCSize                string     `json:"hc_size"`
	PlanType              string     `json:"plan_type"`
	PaymentTerms          string     `json:"payment_terms"`
	FinalPrice            float64    `json:"final_price"`
	LastPaymentDate       *time.Time `json:"last_payment_date"`
	Notes                 string     `json:"notes"`
	CrossSellResumeDate   *time.Time `json:"cross_sell_resume_date"`
	RenewalDate           *time.Time `json:"renewal_date"`
	CreatedAt             time.Time  `json:"created_at"`
	ChurnReason           string     `json:"churn_reason"`
	WAUndeliverable       bool       `json:"wa_undeliverable"`
	FeatureUpdateSent     bool       `json:"feature_update_sent"`
	DaysSinceCSLastSent   int        `json:"days_since_cs_last_sent"`
	FirstTimeDiscountPct  float64    `json:"first_time_discount_pct"`
	NextDiscountPctManual float64    `json:"next_discount_pct_manual"`
	QuotationLinkExpires  *time.Time `json:"quotation_link_expires"`
	AEAssigned            bool       `json:"ae_assigned"`
	UsageScoreAvg30d      int        `json:"usage_score_avg_30d"`
	BackupOwnerTelegramID string     `json:"backup_owner_telegram_id"`
	AETelegramID          string     `json:"ae_telegram_id"`
	BDProspectID          string     `json:"bd_prospect_id"`
	RiskFlag              bool       `json:"risk_flag"`
	WorkspaceID           string     `json:"workspace_id"`
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
	now := time.Now()
	c.LastInteractionDate = &now
}

// GetOwnerWA returns the owner WA number or empty string if nil.
func (c *Client) GetOwnerWA() string {
	if c.OwnerWA == nil {
		return ""
	}
	return *c.OwnerWA
}
