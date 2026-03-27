package dto

// WAWebhookRequest represents the incoming WhatsApp webhook payload from HaloAI
type WAWebhookRequest struct {
	MessageID   string `json:"message_id" example:"msg_1234567890"`
	PhoneNumber string `json:"phone_number" example:"6281234567890"`
	MessageType string `json:"message_type" example:"text"` // text, image, voice, video, document
	Text        string `json:"text" example:"Thank you for the reminder."`
	Timestamp   string `json:"timestamp" example:"2025-03-26T10:30:00Z"`
}
