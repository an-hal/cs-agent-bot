package entity

import "time"

type Escalation struct {
	EscalationID        string     `json:"escalation_id"`
	CompanyID           string     `json:"company_id"`
	EscID               string     `json:"esc_id"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
	ResolvedAt          *time.Time `json:"resolved_at,omitempty"`
	Priority            string     `json:"priority"`
	Reason              string     `json:"reason"`
	NotifiedParty       string     `json:"notified_party"`
	TelegramMessageSent string     `json:"telegram_message_sent"`
	ResolvedBy          string     `json:"resolved_by"`
	EscNotes            string     `json:"notes"`
	WorkspaceID         string     `json:"workspace_id"`
}

const (
	EscalationStatusOpen     = "Open"
	EscalationStatusResolved = "Resolved"
)

const (
	EscInvoiceOverdue15 = "ESC-001"
	EscObjection        = "ESC-002"
	EscLowNPS           = "ESC-003"
	EscRen0NoReply      = "ESC-004"
	EscHighValueChurn   = "ESC-005"
	EscAngryClient      = "ESC-006"
	EscPaymentClaim     = "ESC-007"
)

const (
	EscPriorityP0Emergency = "P0 Emergency"
	EscPriorityP1Critical  = "P1 Critical"
	EscPriorityP2High      = "P2 High"
)
