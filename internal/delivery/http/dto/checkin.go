package dto

// CheckinFormRequest represents the request body for check-in form submission
type CheckinFormRequest struct {
	CompanyID string `json:"company_id" example:"KTK-001" validate:"required"`
}
