package webhook

import "time"

type WAWebhookPayload struct {
	MessageID   string    `json:"message_id"`
	PhoneNumber string    `json:"phone_number"`
	MessageType string    `json:"message_type"` // text, image, voice, video, document
	Text        string    `json:"text"`
	Timestamp   time.Time `json:"timestamp"`
}

type NewClientPayload struct {
	CompanyID       string `json:"company_id" validate:"required"`
	CompanyName     string `json:"company_name" validate:"required"`
	PICName         string `json:"pic_name" validate:"required"`
	PICWA           string `json:"pic_wa" validate:"required"`
	OwnerName       string `json:"owner_name"`
	OwnerWA         string `json:"owner_wa"`
	Segment         string `json:"segment" validate:"required"`
	ContractMonths  int    `json:"contract_months" validate:"required"`
	ContractStart   string `json:"contract_start" validate:"required"`
	ContractEnd     string `json:"contract_end" validate:"required"`
	ActivationDate  string `json:"activation_date"`
	OwnerTelegramID string `json:"owner_telegram_id" validate:"required"`
}

type CheckinFormPayload struct {
	CompanyID string `json:"company_id" validate:"required"`
}
