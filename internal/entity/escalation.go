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

	// feat/08 extensions
	Severity       string     `json:"severity,omitempty"`
	AssignedTo     string     `json:"assigned_to,omitempty"`
	ResolutionNote string     `json:"resolution_note,omitempty"`
	NotifiedVia    string     `json:"notified_via,omitempty"`
	NotifiedAt     *time.Time `json:"notified_at,omitempty"`
	MasterDataID   string     `json:"master_data_id,omitempty"`
}

// EscalationFilter holds optional filters for listing escalations.
type EscalationFilter struct {
	WorkspaceIDs []string // one or more workspace UUIDs (holding workspaces expand to member IDs)
	CompanyID    string
	Search       string // ILIKE across company_id, reason, esc_id, notes
	Status       string // exact match
	Priority     string // exact match
	Severity     string // exact match (feat/08)
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

const (
	EscSeverityCritical = "critical"
	EscSeverityHigh     = "high"
	EscSeverityMedium   = "medium"
)
