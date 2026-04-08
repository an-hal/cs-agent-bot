package entity

import "time"

// Template represents a WhatsApp message template with variable substitution.
type Template struct {
	TemplateID       string    `json:"template_id" db:"template_id"`
	TemplateName     string    `json:"template_name" db:"template_name"`
	TemplateContent  string    `json:"template_content" db:"template_content"`
	TemplateCategory string    `json:"template_category" db:"template_category"`
	Language         string    `json:"language" db:"language"`
	Active           bool      `json:"active" db:"active"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// TemplateFilter holds optional filters for listing templates.
type TemplateFilter struct {
	Category string
	Language string
	Active   *bool // nil = all, true/false = filter
}

// Template Category constants
const (
	TemplateCategoryRenewal    = "renewal"
	TemplateCategoryCheckin    = "checkin"
	TemplateCategoryInvoice    = "invoice"
	TemplateCategoryOverdue    = "overdue"
	TemplateCategoryNPS        = "nps"
	TemplateCategoryReferral   = "referral"
	TemplateCategoryHealth     = "health"
	TemplateCategoryCrossSell  = "cross_sell"
	TemplateCategoryExpansion  = "expansion"
	TemplateCategoryEscalation = "escalation"
)

// Template Language constants
const (
	LanguageID = "id"
	LanguageEN = "en"
)

// Template ID constants
const (
	// Renewal templates
	TemplateRenewal60D = "renewal_60d"
	TemplateRenewal45D = "renewal_45d"
	TemplateRenewal30D = "renewal_30d"
	TemplateRenewal15D = "renewal_15d"
	TemplateRenewal0D  = "renewal_0d"

	// Check-in templates
	TemplateCheckinA1Form = "checkin_a1_form"
	TemplateCheckinA1Call = "checkin_a1_call"
	TemplateCheckinA2Form = "checkin_a2_form"
	TemplateCheckinA2Call = "checkin_a2_call"
	TemplateCheckinB1Form = "checkin_b1_form"
	TemplateCheckinB1Call = "checkin_b1_call"
	TemplateCheckinB2Form = "checkin_b2_form"
	TemplateCheckinB2Call = "checkin_b2_call"

	// Invoice templates
	TemplateInvoicePre14 = "invoice_pre14"
	TemplateInvoicePre7  = "invoice_pre7"
	TemplateInvoicePre3  = "invoice_pre3"

	// Overdue templates
	TemplateOverduePost1 = "overdue_post1"
	TemplateOverduePost4 = "overdue_post4"
	TemplateOverduePost8 = "overdue_post8"

	// NPS templates
	TemplateNPS1 = "nps1"
	TemplateNPS2 = "nps2"
	TemplateNPS3 = "nps3"

	// Referral template
	TemplateReferral = "referral_request"

	// Health templates
	TemplateLowUsage = "low_usage_alert"
	TemplateLowNPS   = "low_nps_alert"

	// Cross-sell templates
	TemplateCrossSellH7  = "cross_sell_h7"
	TemplateCrossSellH14 = "cross_sell_h14"
	TemplateCrossSellH21 = "cross_sell_h21"
	TemplateCrossSellH30 = "cross_sell_h30"
	TemplateCrossSellLT1 = "cross_sell_lt1"
	TemplateCrossSellLT2 = "cross_sell_lt2"
	TemplateCrossSellLT3 = "cross_sell_lt3"
)
