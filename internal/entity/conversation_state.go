package entity

import "time"

// ActiveFlow constants
const (
	FlowRenewal   = "RENEWAL"
	FlowInvoice   = "INVOICE"
	FlowCheckIn   = "CHECKIN"
	FlowCrossSell = "CROSS_SELL"
	FlowNPS       = "NPS"
)

// ResponseClassification constants
const (
	RespPositive         = "POSITIVE"
	RespObjectionPrice   = "OBJECTION_PRICE"
	RespObjectionFeature = "OBJECTION_FEATURE"
	RespDelay            = "DELAY"
	RespReject           = "REJECT"
	RespAngry            = "ANGRY"
	RespOOO              = "OOO"
	RespPaid             = "PAID"
	RespNPS              = "NPS"
)

// ConversationState holds the per-company conversation tracking state.
// Nullable DB columns use Go pointer types so that sql.Scan can handle NULL.
type ConversationState struct {
	CompanyID              string     `json:"company_id"`
	CompanyName            *string    `json:"company_name"`
	ActiveFlow             *string    `json:"active_flow"`
	CurrentStage           *string    `json:"current_stage"`
	LastMessageType        *string    `json:"last_message_type"`
	LastMessageDate        *time.Time `json:"last_message_date"`
	ResponseStatus         string     `json:"response_status"`
	ResponseClassification *string    `json:"response_classification"`
	AttemptCount           int        `json:"attempt_count"`
	CooldownUntil          *time.Time `json:"cooldown_until"`
	BotActive              bool       `json:"bot_active"`
	ReasonBotPaused        *string    `json:"reason_bot_paused"`
	NextScheduledAction    *string    `json:"next_scheduled_action"`
	NextScheduledDate      *time.Time `json:"next_scheduled_date"`
	HumanOwnerNotified     bool       `json:"human_owner_notified"`
}

// IsOnCooldown checks if the bot is currently in cooldown period
func (cs *ConversationState) IsOnCooldown() bool {
	if cs.CooldownUntil == nil {
		return false
	}
	return time.Now().Before(*cs.CooldownUntil)
}

// ShouldSend checks if bot should send message (anti-spam + cooldown + bot_active)
func (cs *ConversationState) ShouldSend() bool {
	if !cs.BotActive {
		return false
	}
	if cs.IsOnCooldown() {
		return false
	}
	return true
}

// IncrementAttempt increases the attempt count
func (cs *ConversationState) IncrementAttempt() {
	cs.AttemptCount++
}

// SetCooldown sets a cooldown period
func (cs *ConversationState) SetCooldown(duration time.Duration) {
	t := time.Now().Add(duration)
	cs.CooldownUntil = &t
}

// RecordMessage records a sent message
func (cs *ConversationState) RecordMessage(messageType, templateID string) {
	cs.LastMessageType = &templateID
	now := time.Now()
	cs.LastMessageDate = &now
	cs.IncrementAttempt()
}

// StringPtr is a helper to create a *string from a string literal.
func StringPtr(v string) *string { return &v }

// GetReasonBotPaused returns the reason the bot is paused, or empty string if nil.
func (cs *ConversationState) GetReasonBotPaused() string {
	if cs.ReasonBotPaused == nil {
		return ""
	}
	return *cs.ReasonBotPaused
}
