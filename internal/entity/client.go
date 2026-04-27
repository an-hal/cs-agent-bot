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
	ContractMonths      int        `json:"contract_months"`
	ContractStart       time.Time  `json:"contract_start"`
	ContractEnd         time.Time  `json:"contract_end"`
	ActivationDate      time.Time  `json:"activation_date"`
	PaymentStatus       string     `json:"payment_status"`
	BotActive           bool       `json:"bot_active"`
	Blacklisted         bool       `json:"blacklisted"`
	OwnerTelegramID     string     `json:"owner_telegram_id"`
	SequenceCS          string     `json:"sequence_cs"`
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
	PaymentTerms          string     `json:"payment_terms"`
	FinalPrice            float64    `json:"final_price"`
	LastPaymentDate       *time.Time `json:"last_payment_date"`
	Notes                 string     `json:"notes"`
	CreatedAt             time.Time  `json:"created_at"`
	FeatureUpdateSent     bool       `json:"feature_update_sent"`
	DaysSinceCSLastSent   int        `json:"days_since_cs_last_sent"`
	AEAssigned            bool       `json:"ae_assigned"`
	BackupOwnerTelegramID string     `json:"backup_owner_telegram_id"`
	AETelegramID          string     `json:"ae_telegram_id"`
	WorkspaceID           string     `json:"workspace_id"`
}

// ClientFilter holds optional filters for listing clients.
type ClientFilter struct {
	WorkspaceIDs  []string // one or more workspace UUIDs (holding workspaces expand to member IDs)
	Search        string   // ILIKE across company_name, pic_name, pic_wa, pic_email, owner_name
	Segment       string   // exact match
	PaymentStatus string   // exact match
	SequenceCS    string   // exact match
	PlanType      string   // exact match
	BotActive     *bool    // nil = all, true/false = filter
}

const (
	SegmentHigh = "High"
	SegmentMid  = "Mid"
	SegmentLow  = "Low"
)

const (
	PaymentStatusPaid    = "Paid"
	PaymentStatusPending = "Pending"
	PaymentStatusOverdue = "Overdue"
	PaymentStatusPartial = "Partial"
)

const (
	SequenceCSActive    = "ACTIVE"
	SequenceCSLongterm  = "LONGTERM"
	SequenceCSSnoozed   = "SNOOZED"
	SequenceCSRejected  = "REJECTED"
	SequenceCSConverted = "CONVERTED"
)

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
