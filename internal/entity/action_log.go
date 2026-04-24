package entity

import "time"

type ActionLog struct {
	Timestamp              time.Time  `json:"timestamp"`
	CompanyID              string     `json:"company_id"`
	CompanyName            string     `json:"company_name"`
	TriggerType            string     `json:"trigger_type"`
	TemplateID             string     `json:"template_id"`
	Channel                string     `json:"channel"`
	MessageSent            bool       `json:"message_sent"`
	ResponseReceived       bool       `json:"response_received"`
	ResponseClassification string     `json:"response_classification"`
	NextActionTriggered    string     `json:"next_action_triggered"`
	LogNotes               string     `json:"log_notes"`
	ReplyTimestamp         *time.Time `json:"reply_timestamp"`
	ReplyText              string     `json:"reply_text"`
	AENotified             bool       `json:"ae_notified"`
	WorkspaceID            string     `json:"workspace_id"`
}

const (
	ChannelWhatsApp = "whatsapp"
	ChannelTelegram = "telegram"
)

// ActionLogSummary aggregates bot-action metrics for the dashboard sidebar.
// All counts are scoped to the `since` cutoff passed to GetActionLogSummary.
type ActionLogSummary struct {
	Total            int64 `json:"total"`              // total bot actions in window
	MessagesSent     int64 `json:"messages_sent"`       // subset where message_sent=true
	RepliesReceived  int64 `json:"replies_received"`    // subset where response_received=true
	EscalationsFired int64 `json:"escalations_fired"`   // subset where next_action_triggered='escalate'
	AENotifications  int64 `json:"ae_notifications"`    // subset where ae_notified=true
	// Convenience derived metric — replies / messages_sent * 100. 0 when no messages_sent.
	ReplyRatePct float64 `json:"reply_rate_pct"`
}
