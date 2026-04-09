package template

import (
	"fmt"
	"math"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// VariableProvider resolves a template variable to its string value.
type VariableProvider func(client entity.Client, invoice *entity.Invoice, cfg TemplateConfig) string

// VariableInfo describes a registered template variable for the API endpoint.
type VariableInfo struct {
	VarKey      string   `json:"var_key"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Source      string   `json:"source"`
	SampleValue string   `json:"sample_value"`
	Channels    []string `json:"channels"`
	Syntax      string   `json:"syntax"`
}

// variableProviders is the hardcoded registry of all available template variables.
// Each entry maps a variable key to its provider function.
// Adding a new variable = add 1 entry here.
var variableProviders = map[string]VariableProvider{
	"company_name": func(c entity.Client, _ *entity.Invoice, _ TemplateConfig) string {
		return c.CompanyName
	},
	"company_id": func(c entity.Client, _ *entity.Invoice, _ TemplateConfig) string {
		return c.CompanyID
	},
	"pic_name": func(c entity.Client, _ *entity.Invoice, _ TemplateConfig) string {
		return c.PICName
	},
	"owner_name": func(c entity.Client, _ *entity.Invoice, _ TemplateConfig) string {
		return c.OwnerName
	},
	"owner_wa": func(c entity.Client, _ *entity.Invoice, _ TemplateConfig) string {
		return c.GetOwnerWA()
	},
	"link_quotation": func(c entity.Client, _ *entity.Invoice, _ TemplateConfig) string {
		return c.QuotationLink
	},
	"link_survey": func(c entity.Client, _ *entity.Invoice, cfg TemplateConfig) string {
		if cfg.SurveyPlatformURL == "" {
			return ""
		}
		return cfg.SurveyPlatformURL + "?cid=" + c.CompanyID
	},
	"link_checkin_form": func(c entity.Client, _ *entity.Invoice, cfg TemplateConfig) string {
		if cfg.CheckinFormURL == "" {
			return ""
		}
		return cfg.CheckinFormURL + "?cid=" + c.CompanyID
	},
	"months_active": func(c entity.Client, _ *entity.Invoice, _ TemplateConfig) string {
		if c.ContractStart.IsZero() {
			return "0"
		}
		monthsActive := int(math.Floor(time.Since(c.ContractStart).Hours() / (24 * 30)))
		return fmt.Sprintf("%d", monthsActive)
	},
	"due_date": func(_ entity.Client, inv *entity.Invoice, _ TemplateConfig) string {
		if inv == nil {
			return ""
		}
		return inv.DueDate.Format("2 Jan 2006")
	},
	"invoice_id": func(_ entity.Client, inv *entity.Invoice, _ TemplateConfig) string {
		if inv == nil {
			return ""
		}
		return inv.InvoiceID
	},
	"benefit_referral": func(_ entity.Client, _ *entity.Invoice, cfg TemplateConfig) string {
		return cfg.ReferralBenefit
	},
}

// variableInfoRegistry contains the metadata for all available template variables.
// This is what the GET /api/template-variables endpoint returns.
var variableInfoRegistry = []VariableInfo{
	{VarKey: "company_name", DisplayName: "Company Name", Description: "The company name from client record", Source: "client", SampleValue: "PT Sejuta Cita", Channels: []string{"wa", "email"}, Syntax: "{company_name}"},
	{VarKey: "company_id", DisplayName: "Company ID", Description: "The company ID", Source: "client", SampleValue: "COMP-001", Channels: []string{"wa", "email"}, Syntax: "{company_id}"},
	{VarKey: "pic_name", DisplayName: "PIC Name", Description: "PIC contact name", Source: "client", SampleValue: "Budi Santoso", Channels: []string{"wa", "email"}, Syntax: "{pic_name}"},
	{VarKey: "owner_name", DisplayName: "Account Owner", Description: "Account owner/AE name", Source: "client", SampleValue: "Ahmad", Channels: []string{"wa", "email"}, Syntax: "{owner_name}"},
	{VarKey: "owner_wa", DisplayName: "Owner WhatsApp", Description: "Account owner WhatsApp number", Source: "client", SampleValue: "628123456789", Channels: []string{"wa"}, Syntax: "{owner_wa}"},
	{VarKey: "link_quotation", DisplayName: "Quotation Link", Description: "Link to the quotation document", Source: "client", SampleValue: "https://example.com/quotation/123", Channels: []string{"wa", "email"}, Syntax: "{link_quotation}"},
	{VarKey: "link_survey", DisplayName: "Survey Link", Description: "NPS survey link with company ID appended", Source: "config", SampleValue: "https://survey.example.com?cid=COMP-001", Channels: []string{"wa", "email"}, Syntax: "{link_survey}"},
	{VarKey: "link_checkin_form", DisplayName: "Check-in Form Link", Description: "Check-in feedback form link with company ID appended", Source: "config", SampleValue: "https://checkin.example.com?cid=COMP-001", Channels: []string{"wa", "email"}, Syntax: "{link_checkin_form}"},
	{VarKey: "months_active", DisplayName: "Months Active", Description: "Number of months since contract start date", Source: "computed", SampleValue: "6", Channels: []string{"wa", "email"}, Syntax: "{months_active}"},
	{VarKey: "due_date", DisplayName: "Invoice Due Date", Description: "Invoice due date formatted as '2 Jan 2006'", Source: "invoice", SampleValue: "15 Jan 2026", Channels: []string{"wa", "email"}, Syntax: "{due_date}"},
	{VarKey: "invoice_id", DisplayName: "Invoice ID", Description: "Invoice identifier", Source: "invoice", SampleValue: "INV-2026-COMP-001", Channels: []string{"wa", "email"}, Syntax: "{invoice_id}"},
	{VarKey: "benefit_referral", DisplayName: "Referral Benefit", Description: "Referral benefit description from config", Source: "config", SampleValue: "diskon 10% selama 3 bulan", Channels: []string{"wa", "email"}, Syntax: "{benefit_referral}"},
}

// GetAvailableVariables returns the list of all registered template variables.
func GetAvailableVariables() []VariableInfo {
	return variableInfoRegistry
}

// GetAvailableVariablesByChannel returns variables filtered by channel.
func GetAvailableVariablesByChannel(channel string) []VariableInfo {
	var result []VariableInfo
	for _, v := range variableInfoRegistry {
		for _, ch := range v.Channels {
			if ch == channel {
				result = append(result, v)
				break
			}
		}
	}
	return result
}

// ValidateTemplateVariables checks that all {VarName} in content are registered.
// Returns a list of unrecognized variable keys.
func ValidateTemplateVariables(content string) []string {
	var unknown []string
	// Simple parser: find all {xxx} patterns
	i := 0
	for i < len(content) {
		start := indexOf(content, '{', i)
		if start == -1 {
			break
		}
		end := indexOf(content, '}', start+1)
		if end == -1 {
			break
		}
		varKey := content[start+1 : end]
		if _, ok := variableProviders[varKey]; !ok {
			unknown = append(unknown, varKey)
		}
		i = end + 1
	}
	return unknown
}

func indexOf(s string, c byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
