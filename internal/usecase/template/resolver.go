package template

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

type TemplateResolver interface {
	ResolveTemplate(ctx context.Context, templateID string, client entity.Client, invoice *entity.Invoice, cfg TemplateConfig) (string, error)
	ResolveEscalationTemplate(ctx context.Context, escID string, client entity.Client, esc entity.Escalation, extraVars map[string]string) (string, error)
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

	// Replace all {bracket} variables using the variable registry
	for varKey, provider := range variableProviders {
		placeholder := "{" + varKey + "}"
		if strings.Contains(body, placeholder) {
			body = strings.ReplaceAll(body, placeholder, provider(client, invoice, cfg))
		}
	}

	// Guard: reject send if any {variable} remains unresolved
	if strings.Contains(body, "{") && strings.Contains(body, "}") {
		return "", fmt.Errorf("unresolved variable in template %s", templateID)
	}

	return body, nil
}

func (r *templateResolver) ResolveEscalationTemplate(ctx context.Context, escID string, client entity.Client, esc entity.Escalation, extraVars map[string]string) (string, error) {
	tmpl, err := r.configRepo.GetEscalationTemplate(ctx, escID)
	if err != nil {
		return "", fmt.Errorf("escalation template not found: %s: %w", escID, err)
	}

	body := tmpl.TelegramMsg

	// Replace escalation-specific variables
	body = strings.ReplaceAll(body, "{esc_id}", esc.EscID)
	body = strings.ReplaceAll(body, "{priority}", esc.Priority)
	body = strings.ReplaceAll(body, "{reason}", esc.Reason)
	body = strings.ReplaceAll(body, "{status}", esc.Status)

	// Replace client variables
	body = strings.ReplaceAll(body, "{company_name}", client.CompanyName)
	body = strings.ReplaceAll(body, "{company_id}", client.CompanyID)
	body = strings.ReplaceAll(body, "{pic_name}", client.PICName)
	body = strings.ReplaceAll(body, "{owner_name}", client.OwnerName)
	body = strings.ReplaceAll(body, "{owner_wa}", client.GetOwnerWA())

	// Replace extra variables (e.g., {Verified_By}, {Invoice_ID})
	for key, value := range extraVars {
		body = strings.ReplaceAll(body, "{"+key+"}", value)
	}

	// Guard: reject if any {variable} remains unresolved
	if strings.Contains(body, "{") && strings.Contains(body, "}") {
		return "", fmt.Errorf("unresolved variable in escalation template %s", escID)
	}

	return body, nil
}
