package entity

import "time"

type Escalation struct {
	EscalationID string     `json:"escalation_id"`
	CompanyID    string     `json:"company_id"`
	EscID        string     `json:"esc_id"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	Priority     string     `json:"priority"`
	Reason       string     `json:"reason"`
}

// Escalation status constants
const (
	EscalationStatusOpen     = "Open"
	EscalationStatusResolved = "Resolved"
)

// Escalation ID constants
const (
	EscInvoiceOverdue15  = "ESC-001"
	EscObjection         = "ESC-002"
	EscLowNPS            = "ESC-003"
	EscRen0NoReply       = "ESC-004"
	EscHighValueChurn    = "ESC-005"
	EscAngryClient       = "ESC-006"
)

// Escalation priority constants
const (
	EscPriorityP0Emergency = "P0 Emergency"
	EscPriorityP1Critical  = "P1 Critical"
	EscPriorityP2High      = "P2 High"
)
