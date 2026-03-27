package repository

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database/sheets"
	"github.com/Sejutacita/cs-agent-bot/internal/service/cache"
	"github.com/rs/zerolog"
)

type ConfigRepository interface {
	GetAllTemplates(ctx context.Context) ([]entity.TriggerTemplate, error)
	GetTemplateByID(ctx context.Context, templateID string) (*entity.TriggerTemplate, error)
	GetEscalationTemplate(ctx context.Context, escID string) (*entity.EscalationTemplate, error)
}

type configRepo struct {
	sheetsClient *sheets.SheetsClient
	cache        cache.SheetCache
	logger       zerolog.Logger
}

func NewConfigRepo(sheetsClient *sheets.SheetsClient, cache cache.SheetCache, logger zerolog.Logger) ConfigRepository {
	return &configRepo{
		sheetsClient: sheetsClient,
		cache:        cache,
		logger:       logger,
	}
}

func (r *configRepo) GetAllTemplates(ctx context.Context) ([]entity.TriggerTemplate, error) {
	rows, err := r.cache.Get(ctx, cache.KeyTriggerTemplate, cache.RangeTriggerTemplate, cache.TTLTriggerTemplate)
	if err != nil {
		return nil, err
	}

	var templates []entity.TriggerTemplate
	// Skip first 5 rows (indices 0-4): 4 info rows + 1 column header row
	for i, row := range rows {
		if i < 5 {
			continue
		}
		t, err := parseTemplateRow(row)
		if err != nil {
			r.logger.Warn().Err(err).Int("row", i+1).Msg("Failed to parse template row")
			continue
		}
		templates = append(templates, *t)
	}

	return templates, nil
}

func (r *configRepo) GetTemplateByID(ctx context.Context, templateID string) (*entity.TriggerTemplate, error) {
	templates, err := r.GetAllTemplates(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range templates {
		if t.TemplateID == templateID {
			return &t, nil
		}
	}

	return nil, fmt.Errorf("template not found: %s", templateID)
}

func (r *configRepo) GetEscalationTemplate(ctx context.Context, escID string) (*entity.EscalationTemplate, error) {
	// Fetch from Google Sheets Sheet 6 (Escalation Rules)
	templates, err := r.getAllEscalationTemplates(ctx)
	if err != nil {
		return nil, err
	}

	for _, t := range templates {
		if t.EscID == escID {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("escalation template not found: %s", escID)
}

func (r *configRepo) getAllEscalationTemplates(ctx context.Context) ([]entity.EscalationTemplate, error) {
	rows, err := r.cache.Get(ctx, cache.KeyEscalation, cache.RangeEscalation, cache.TTLEscalation)
	if err != nil {
		return nil, err
	}

	var templates []entity.EscalationTemplate
	// Skip first row (index 0): header row
	for i, row := range rows {
		if i == 0 {
			continue // skip header row
		}
		t, err := parseEscalationTemplateRow(row)
		if err != nil {
			r.logger.Warn().Err(err).Int("row", i+1).Msg("Failed to parse escalation row")
			continue
		}
		templates = append(templates, *t)
	}

	return templates, nil
}

// parseEscalationTemplateRow maps a Sheets row to EscalationTemplate
// Column order based on Sheet 6: Escalation Rules
// Row 1 (index 0) contains column names. Data starts at row 2 (index 1).
func parseEscalationTemplateRow(row []interface{}) (*entity.EscalationTemplate, error) {
	// if len(row) < 6 {
	// 	return nil, fmt.Errorf("escalation row too short: %d columns, expected at least 6", len(row))
	// }

	return &entity.EscalationTemplate{
		EscID:       safeString(row, 0), // Escalation_ID
		Name:        safeString(row, 1), // Trigger_Condition (use as name for now)
		Priority:    safeString(row, 3), // Priority
		TelegramMsg: safeString(row, 5), // Telegram Message to Owner
	}, nil
}

func parseTemplateRow(row []interface{}) (*entity.TriggerTemplate, error) {
	if len(row) < 5 {
		return nil, fmt.Errorf("template row too short: %d columns", len(row))
	}

	return &entity.TriggerTemplate{
		TemplateID:  safeString(row, 4),
		TriggerType: safeString(row, 1),
		Condition:   safeString(row, 3),
		Body:        safeString(row, 5),
		Channel:     safeString(row, 4),
	}, nil
}
