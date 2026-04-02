package dto

// WAWebhookRequest represents the incoming WhatsApp webhook payload from HaloAI
type WAWebhookRequest struct {
	Event     string      `json:"event"`
	Customer  CustomerDTO `json:"customer"`
	UTM       UTMDOT      `json:"utm"`
	Trigger   TriggerDTO  `json:"trigger"`
	RoomID    string      `json:"room_id"`
	Message   MessageDTO  `json:"message"`
	RequestID string      `json:"request_id"`
}

type CustomerDTO struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type UTMDOT struct {
	Source   *string `json:"utm_source"`
	Medium   *string `json:"utm_medium"`
	Campaign *string `json:"utm_campaign"`
}

type TriggerDTO struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	MessageID string `json:"messageId"`
	Timestamp int64  `json:"timestamp"`
}

type MessageDTO struct {
	FromMe    bool   `json:"from_me"`
	Body      string `json:"body"`
	HasMedia  bool   `json:"has_media"`
	Timestamp int64  `json:"timestamp"`
	Ack       int    `json:"ack"`
}
