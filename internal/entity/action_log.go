package entity

import "time"

type ActionLog struct {
	LogID       string    `json:"log_id"`
	CompanyID   string    `json:"company_id"`
	Timestamp   time.Time `json:"timestamp"`
	TriggerType string    `json:"trigger_type"`
	MessageID   string    `json:"message_id"`
	Channel     string    `json:"channel"`
	Details     string    `json:"details"`
}

// Channel constants
const (
	ChannelWhatsApp = "whatsapp"
	ChannelTelegram = "telegram"
)
