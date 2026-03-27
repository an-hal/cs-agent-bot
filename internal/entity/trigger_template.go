package entity

type TriggerTemplate struct {
	TemplateID  string `json:"template_id"`
	TriggerType string `json:"trigger_type"`
	Condition   string `json:"condition"`
	Body        string `json:"body"`
	Channel     string `json:"channel"`
}

// EscalationTemplate represents escalation Telegram message templates
type EscalationTemplate struct {
	EscID       string `json:"esc_id"`       // ESC-001, ESC-002, etc.
	Name        string `json:"name"`         // Human readable name
	Priority    string `json:"priority"`     // P0, P1, P2
	TelegramMsg string `json:"telegram_msg"` // Template body with variables
}
