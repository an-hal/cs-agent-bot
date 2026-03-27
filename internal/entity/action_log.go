package entity

import "time"

type ActionLog struct {
	Timestamp   time.Time `json:"timestamp"`
	CompanyID   string    `json:"company_id"`
	CompanyName string    `json:"company_name"`
	TriggerType string    `json:"trigger_type"`
	TemplateID  string    `json:"template_id"`
	Channel     string    `json:"channel"`
	Details     string    `json:"details"`
}

// Channel constants
const (
	ChannelWhatsApp = "whatsapp"
	ChannelTelegram = "telegram"
)
