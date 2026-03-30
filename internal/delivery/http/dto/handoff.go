package dto

// HandoffCreateRequest represents the request body for client handoff from BD team
type HandoffCreateRequest struct {
	CompanyID       string `json:"company_id" example:"KTK-001" validate:"required"`
	CompanyName     string `json:"company_name" example:"PT Maju Bersama Sejahtera" validate:"required"`
	PICName         string `json:"pic_name" example:"Budi Santoso" validate:"required"`
	PICWA           string `json:"pic_wa" example:"6281234567890" validate:"required"`
	OwnerName       string `json:"owner_name" example:"Rina" validate:"required"`
	OwnerWA         string `json:"owner_wa" example:"6289876543210"`
	Segment         string `json:"segment" example:"Mid" validate:"required"`
	ContractMonths  int    `json:"contract_months" example:"12" validate:"required"`
	ContractStart   string `json:"contract_start" example:"2024-01-15" validate:"required"`
	ContractEnd     string `json:"contract_end" example:"2026-01-15" validate:"required"`
	ActivationDate  string `json:"activation_date" example:"2024-01-15"`
	OwnerTelegramID string `json:"owner_telegram_id" example:"-1001234567890" validate:"required"`
}
