package template

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

type TemplateResolver interface {
	ResolveTemplate(ctx context.Context, templateID string, client entity.Client, invoice *entity.Invoice, cfg TemplateConfig) (string, error)
}

type TemplateConfig struct {
	SurveyPlatformURL string
	CheckinFormURL    string
	ReferralBenefit   string
}

type templateResolver struct {
	configRepo repository.ConfigRepository
	logger     zerolog.Logger
}

func NewTemplateResolver(configRepo repository.ConfigRepository, logger zerolog.Logger) TemplateResolver {
	return &templateResolver{
		configRepo: configRepo,
		logger:     logger,
	}
}

func (r *templateResolver) ResolveTemplate(ctx context.Context, templateID string, client entity.Client, invoice *entity.Invoice, cfg TemplateConfig) (string, error) {
	tmpl, err := r.configRepo.GetTemplateByID(ctx, templateID)
	if err != nil {
		return "", fmt.Errorf("template not found: %s: %w", templateID, err)
	}

	body := tmpl.Body

	// Replace all [bracket] variables
	body = strings.ReplaceAll(body, "[Company_Name]", client.CompanyName)
	body = strings.ReplaceAll(body, "[PIC_Name]", client.PICName)
	body = strings.ReplaceAll(body, "[Owner_Name]", client.OwnerName)
	body = strings.ReplaceAll(body, "[Owner_WA]", client.OwnerWA)
	body = strings.ReplaceAll(body, "[link_quotation]", client.QuotationLink)

	// Survey link
	if cfg.SurveyPlatformURL != "" {
		body = strings.ReplaceAll(body, "[link_survey]", cfg.SurveyPlatformURL+"?cid="+client.CompanyID)
	}

	// Checkin form link
	if cfg.CheckinFormURL != "" {
		body = strings.ReplaceAll(body, "[link_checkin_form]", cfg.CheckinFormURL+"?cid="+client.CompanyID)
	}

	// Months active
	if !client.ContractStart.IsZero() {
		monthsActive := int(math.Floor(time.Since(client.ContractStart).Hours() / (24 * 30)))
		body = strings.ReplaceAll(body, "[months_active]", fmt.Sprintf("%d", monthsActive))
	}

	// Invoice-specific variables
	if invoice != nil {
		body = strings.ReplaceAll(body, "[Due_Date]", invoice.DueDate.Format("2 Jan 2006"))
		body = strings.ReplaceAll(body, "[Invoice_ID]", invoice.InvoiceID)
	}

	// Referral benefit
	body = strings.ReplaceAll(body, "[Benefit_Referral]", cfg.ReferralBenefit)

	// Guard: reject send if any [variable] remains unresolved
	if strings.Contains(body, "[") && strings.Contains(body, "]") {
		return "", fmt.Errorf("unresolved variable in template %s", templateID)
	}

	return body, nil
}
