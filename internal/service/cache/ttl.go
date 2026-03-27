package cache

import "time"

const (
	TTLTriggerTemplate   = 5 * time.Minute
	TTLMasterClient      = 2 * time.Minute
	TTLInvoices          = 2 * time.Minute
	TTLConversationState = 1 * time.Minute
	TTLEscalation        = 1 * time.Minute
)

// Redis key constants
const (
	KeyTriggerTemplate   = "sheet:trigger_template"
	KeyMasterClient      = "sheet:master_client"
	KeyInvoices          = "sheet:invoices"
	KeyConversationState = "sheet:conv_state"
	KeyEscalation        = "sheet:escalation"
)

// Sheet range constants (tab names)
const (
	RangeMasterClient      = "'1. Master Client'"
	RangeInvoices          = "'2. Invoice & Billing'"
	RangeTriggerTemplate   = "'3. Trigger & Template'"
	RangeConversationState = "'4. Conversation State'"
	RangeActionLog         = "'5. Action Log'"
	RangeEscalation        = "'6. Escalation Rules'"
)
