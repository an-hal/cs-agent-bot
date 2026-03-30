package entity

import "time"

type ActionLog struct {
	Timestamp              time.Time `json:"timestamp"`
	CompanyID              string    `json:"company_id"`
	CompanyName            string    `json:"company_name"`
	TriggerType            string    `json:"trigger_type"`
	TemplateID             string    `json:"template_id"`
	Channel                string    `json:"channel"`
	MessageSent            bool      `json:"message_sent"`
	ResponseReceived       bool      `json:"response_received"`
	ResponseClassification string    `json:"response_classification"`
	NextActionTriggered    string    `json:"next_action_triggered"`
	LogNotes               string    `json:"log_notes"`
}

// Channel constants
const (
	ChannelWhatsApp = "whatsapp"
	ChannelTelegram = "telegram"
)
