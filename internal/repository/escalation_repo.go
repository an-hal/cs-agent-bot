package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database/sheets"
	"github.com/Sejutacita/cs-agent-bot/internal/service/cache"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type EscalationRepository interface {
	GetOpenByCompanyAndEscID(ctx context.Context, companyID, escID string) (*entity.Escalation, error)
	OpenEscalation(ctx context.Context, esc entity.Escalation) error
}

type escalationRepo struct {
	sheetsClient *sheets.SheetsClient
	cache        cache.SheetCache
	logger       zerolog.Logger
}

func NewEscalationRepo(sheetsClient *sheets.SheetsClient, cache cache.SheetCache, logger zerolog.Logger) EscalationRepository {
	return &escalationRepo{
		sheetsClient: sheetsClient,
		cache:        cache,
		logger:       logger,
	}
}

func (r *escalationRepo) GetOpenByCompanyAndEscID(ctx context.Context, companyID, escID string) (*entity.Escalation, error) {
	rows, err := r.cache.Get(ctx, cache.KeyEscalation, cache.RangeEscalation, cache.TTLEscalation)
	if err != nil {
		return nil, err
	}

	// Skip first 3 rows (indices 0-2): 2 info rows + 1 column header row
	for i, row := range rows {
		if i < 3 {
			continue
		}
		if safeString(row, 1) == companyID &&
			safeString(row, 2) == escID &&
			safeString(row, 3) == entity.EscalationStatusOpen {
			return parseEscalationRow(row)
		}
	}

	return nil, nil // no open escalation found, not an error
}

func (r *escalationRepo) OpenEscalation(ctx context.Context, esc entity.Escalation) error {
	if esc.EscalationID == "" {
		esc.EscalationID = uuid.New().String()
	}
	if esc.CreatedAt.IsZero() {
		esc.CreatedAt = time.Now()
	}
	esc.Status = entity.EscalationStatusOpen

	row := escalationToRow(esc)
	if err := r.sheetsClient.AppendRows(ctx, cache.RangeEscalation, [][]interface{}{row}); err != nil {
		return err
	}
	return r.cache.Invalidate(ctx, cache.KeyEscalation)
}

func parseEscalationRow(row []interface{}) (*entity.Escalation, error) {
	if len(row) < 4 {
		return nil, fmt.Errorf("escalation row too short: %d columns", len(row))
	}

	esc := &entity.Escalation{
		EscalationID: safeString(row, 0),
		CompanyID:    safeString(row, 1),
		EscID:        safeString(row, 2),
		Status:       safeString(row, 3),
		CreatedAt:    safeDate(row, 4),
		Priority:     safeString(row, 6),
		Reason:       safeString(row, 7),
	}

	resolvedAt := safeString(row, 5)
	if resolvedAt != "" {
		t, err := time.Parse("2006-01-02", resolvedAt)
		if err == nil {
			esc.ResolvedAt = &t
		}
	}

	return esc, nil
}

func escalationToRow(esc entity.Escalation) []interface{} {
	resolvedAt := ""
	if esc.ResolvedAt != nil {
		resolvedAt = esc.ResolvedAt.Format("2006-01-02")
	}

	return []interface{}{
		esc.EscalationID,
		esc.CompanyID,
		esc.EscID,
		esc.Status,
		esc.CreatedAt.Format(time.RFC3339),
		resolvedAt,
		esc.Priority,
		esc.Reason,
	}
}
