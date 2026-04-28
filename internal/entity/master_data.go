package entity

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Master Data stages.
const (
	StageLead     = "LEAD"
	StageProspect = "PROSPECT"
	StageClient   = "CLIENT"
	StageDormant  = "DORMANT"
)

// Sequence statuses.
const (
	SeqStatusActive      = "ACTIVE"
	SeqStatusPaused      = "PAUSED"
	SeqStatusNurture     = "NURTURE"
	SeqStatusNurturePool = "NURTURE_POOL"
	SeqStatusSnoozed     = "SNOOZED"
	SeqStatusDormant     = "DORMANT"
)

// Risk flag enum values for the textual risk flag.
const (
	RiskHigh = "High"
	RiskMid  = "Mid"
	RiskLow  = "Low"
	RiskNone = "None"
)

// Custom field types.
const (
	FieldTypeText        = "text"
	FieldTypeNumber      = "number"
	FieldTypeDate        = "date"
	FieldTypeBoolean     = "boolean"
	FieldTypeSelect      = "select"
	FieldTypeMultiSelect = "multi_select"
	FieldTypeURL         = "url"
	FieldTypeEmail       = "email"
	// Phase 5 — generic-CRM types
	FieldTypeMoney      = "money"      // {value: number, currency: ISO4217}
	FieldTypePhone      = "phone"      // E.164 or local format
	FieldTypePercentage = "percentage" // number 0-100, rendered with %
)

// MasterData is the dashboard-facing client record. It is read from the
// `master_data` SQL view (which aliases the existing `clients` table) and
// written via the master_data usecase which targets `clients` directly.
type MasterData struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`

	CompanyID   string `json:"company_id"`
	CompanyName string `json:"company_name"`
	Stage       string `json:"stage"`

	PICName     string `json:"pic_name"`
	PICNickname string `json:"pic_nickname"`
	PICRole     string `json:"pic_role"`
	PICWA       string `json:"pic_wa"`
	PICEmail    string `json:"pic_email"`

	OwnerName       string `json:"owner_name"`
	OwnerWA         string `json:"owner_wa"`
	OwnerTelegramID string `json:"owner_telegram_id"`

	BotActive      bool       `json:"bot_active"`
	Blacklisted    bool       `json:"blacklisted"`
	SequenceStatus string     `json:"sequence_status"`
	SnoozeUntil    *time.Time `json:"snooze_until,omitempty"`
	SnoozeReason   string     `json:"snooze_reason"`

	RiskFlag string `json:"risk_flag"`

	ContractStart   *time.Time `json:"contract_start,omitempty"`
	ContractEnd     *time.Time `json:"contract_end,omitempty"`
	ContractMonths  int        `json:"contract_months"`
	DaysToExpiry    *int       `json:"days_to_expiry,omitempty"`
	PaymentStatus   string     `json:"payment_status"`
	PaymentTerms    string     `json:"payment_terms"`
	FinalPrice      int64      `json:"final_price"`
	LastPaymentDate *time.Time `json:"last_payment_date,omitempty"`
	Renewed         bool       `json:"renewed"`

	// Generic billing model (Phase 5)
	BillingPeriod string   `json:"billing_period"` // monthly|quarterly|annual|one_time|perpetual
	Quantity      *int     `json:"quantity,omitempty"`
	UnitPrice     *float64 `json:"unit_price,omitempty"`
	Currency      string   `json:"currency"` // ISO 4217 code

	LastInteractionDate *time.Time `json:"last_interaction_date,omitempty"`

	Notes string `json:"notes"`

	CustomFields map[string]any `json:"custom_fields"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// WorkspaceName is populated for holding workspace queries only.
	WorkspaceName string `json:"workspace_name,omitempty"`
}

// Core field name mappings for GetField. Returns the string representation
// of a field or ("", false) when not found.
func (m *MasterData) GetField(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "stage":
		return m.Stage, m.Stage != ""
	case "payment_status":
		return m.PaymentStatus, m.PaymentStatus != ""
	case "bot_active":
		return fmt.Sprintf("%v", m.BotActive), true
	case "blacklisted":
		return fmt.Sprintf("%v", m.Blacklisted), true
	case "renewed":
		return fmt.Sprintf("%v", m.Renewed), true
	case "sequence_status":
		return m.SequenceStatus, m.SequenceStatus != ""
	case "risk_flag":
		return m.RiskFlag, m.RiskFlag != ""
	case "company_name":
		return m.CompanyName, m.CompanyName != ""
	case "company_id":
		return m.CompanyID, m.CompanyID != ""
	case "pic_name":
		return m.PICName, m.PICName != ""
	case "pic_wa":
		return m.PICWA, m.PICWA != ""
	case "pic_email":
		return m.PICEmail, m.PICEmail != ""
	case "owner_name":
		return m.OwnerName, m.OwnerName != ""
	case "owner_wa":
		return m.OwnerWA, m.OwnerWA != ""
	case "payment_terms":
		return m.PaymentTerms, m.PaymentTerms != ""
	case "notes":
		return m.Notes, m.Notes != ""
	case "contract_months":
		return fmt.Sprintf("%d", m.ContractMonths), true
	case "final_price":
		return fmt.Sprintf("%d", m.FinalPrice), true
	case "days_to_expiry":
		if m.DaysToExpiry == nil {
			return "", false
		}
		return fmt.Sprintf("%d", *m.DaysToExpiry), true
	case "snooze_reason":
		return m.SnoozeReason, m.SnoozeReason != ""
	}

	// Fall through to custom_fields.
	if m.CustomFields != nil {
		v, ok := m.CustomFields[name]
		if ok {
			return fmt.Sprintf("%v", v), true
		}
	}
	return "", false
}

// GetDateField returns a date-typed field value. Returns (nil, false) when not found.
func (m *MasterData) GetDateField(name string) (*time.Time, bool) {
	switch strings.ToLower(name) {
	case "contract_start":
		return m.ContractStart, m.ContractStart != nil
	case "contract_end":
		return m.ContractEnd, m.ContractEnd != nil
	case "last_payment_date":
		return m.LastPaymentDate, m.LastPaymentDate != nil
	case "last_interaction_date":
		return m.LastInteractionDate, m.LastInteractionDate != nil
	case "snooze_until":
		return m.SnoozeUntil, m.SnoozeUntil != nil
	}

	// Check custom_fields for date strings.
	if m.CustomFields != nil {
		v, ok := m.CustomFields[name]
		if ok {
			s, isStr := v.(string)
			if isStr {
				t, err := time.Parse(time.RFC3339, s)
				if err != nil {
					t, err = time.Parse("2006-01-02", s)
					if err != nil {
						return nil, false
					}
				}
				return &t, true
			}
		}
	}
	return nil, false
}

// MasterDataFilter holds list query parameters.
type MasterDataFilter struct {
	WorkspaceIDs  []string
	Stages        []string
	Search        string
	RiskFlag      string
	BotActive     *bool
	PaymentStatus string
	ExpiryWithin  int // days
	SortBy        string
	SortDir       string
	Offset        int
	Limit         int
}

// CustomFieldDefinition defines a workspace-scoped JSONB field.
type CustomFieldDefinition struct {
	ID             string          `json:"id"`
	WorkspaceID    string          `json:"workspace_id"`
	FieldKey       string          `json:"field_key"`
	FieldLabel     string          `json:"field_label"`
	FieldType      string          `json:"field_type"`
	IsRequired     bool            `json:"is_required"`
	DefaultValue   string          `json:"default_value,omitempty"`
	Placeholder    string          `json:"placeholder,omitempty"`
	Description    string          `json:"description,omitempty"`
	Options        json.RawMessage `json:"options,omitempty" swaggertype:"object"`
	MinValue       *float64        `json:"min_value,omitempty"`
	MaxValue       *float64        `json:"max_value,omitempty"`
	RegexPattern   string          `json:"regex_pattern,omitempty"`
	SortOrder      int             `json:"sort_order"`
	VisibleInTable bool            `json:"visible_in_table"`
	ColumnWidth    int             `json:"column_width"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// SelectOptions returns the parsed select options or nil for non-select fields.
func (c *CustomFieldDefinition) SelectOptions() []string {
	if len(c.Options) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(c.Options, &out); err != nil {
		return nil
	}
	return out
}

// Source tags for MasterDataMutation.Source — helps FE filter edit history
// by origin (dashboard edit vs bot write vs bulk import vs API caller).
const (
	MutationSourceDashboard    = "dashboard"
	MutationSourceBot          = "bot"
	MutationSourceImport       = "import"
	MutationSourceAPI          = "api"
	MutationSourceReactivation = "reactivation"
	MutationSourceHandoff      = "handoff"
)

// MasterDataMutation is one row of dashboard edit history.
type MasterDataMutation struct {
	ID             string         `json:"id"`
	WorkspaceID    string         `json:"workspace_id"`
	MasterDataID   string         `json:"master_data_id"`
	CompanyID      string         `json:"company_id,omitempty"`
	CompanyName    string         `json:"company_name,omitempty"`
	Action         string         `json:"action"`
	Source         string         `json:"source"`
	ActorEmail     string         `json:"actor"`
	ChangedFields  []string       `json:"changed_fields"`
	PreviousValues map[string]any `json:"previous_values,omitempty"`
	NewValues      map[string]any `json:"new_values,omitempty"`
	Note           string         `json:"note,omitempty"`
	Timestamp      time.Time      `json:"timestamp"`
}

// ActionLogWorkflow is one workflow node execution trace (plural action_logs table).
type ActionLogWorkflow struct {
	ID             string          `json:"id"`
	WorkspaceID    string          `json:"workspace_id"`
	MasterDataID   string          `json:"master_data_id"`
	TriggerID      string          `json:"trigger_id"`
	TemplateID     string          `json:"template_id,omitempty"`
	Status         string          `json:"status"`
	Channel        string          `json:"channel,omitempty"`
	Phase          string          `json:"phase,omitempty"`
	FieldsRead     json.RawMessage `json:"fields_read,omitempty" swaggertype:"object"`
	FieldsWritten  json.RawMessage `json:"fields_written,omitempty" swaggertype:"object"`
	Replied        bool            `json:"replied"`
	ConversationID string          `json:"conversation_id,omitempty"`
	Timestamp      time.Time       `json:"timestamp"`
}

// ApprovalRequest is the minimal scaffold for the checker-maker queue.
// Feature 04 will own the full schema.
type ApprovalRequest struct {
	ID              string         `json:"id"`
	WorkspaceID     string         `json:"workspace_id"`
	RequestType     string         `json:"request_type"`
	Description     string         `json:"description"`
	Payload         map[string]any `json:"payload"`
	Status          string         `json:"status"`
	MakerEmail      string         `json:"maker_email"`
	MakerAt         time.Time      `json:"maker_at"`
	CheckerEmail    string         `json:"checker_email,omitempty"`
	CheckerAt       *time.Time     `json:"checker_at,omitempty"`
	RejectionReason string         `json:"rejection_reason,omitempty"`
	ExpiresAt       time.Time      `json:"expires_at"`
	AppliedAt       *time.Time     `json:"applied_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// Approval request types.
const (
	ApprovalTypeDeleteClient        = "delete_client_record"
	ApprovalTypeBulkImport          = "bulk_import_master_data"
	ApprovalTypeStageTransition     = "stage_transition"
	ApprovalTypeIntegrationKeyChange = "integration_key_change"
	ApprovalStatusPending           = "pending"
	ApprovalStatusApproved          = "approved"
	ApprovalStatusRejected          = "rejected"
	ApprovalStatusExpired           = "expired"
)
