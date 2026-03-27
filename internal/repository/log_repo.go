package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database/sheets"
	"github.com/Sejutacita/cs-agent-bot/internal/service/cache"
	"github.com/rs/zerolog"
)

type LogRepository interface {
	AppendLog(ctx context.Context, entry entity.ActionLog) error
	SentTodayAlready(ctx context.Context, companyID string) (bool, error)
	MessageIDExists(ctx context.Context, messageID string) (bool, error)
}

type logRepo struct {
	sheetsClient *sheets.SheetsClient
	logger       zerolog.Logger
}

func NewLogRepo(sheetsClient *sheets.SheetsClient, logger zerolog.Logger) LogRepository {
	return &logRepo{
		sheetsClient: sheetsClient,
		logger:       logger,
	}
}

// AppendLog appends a new entry to the action log. Never cached.
func (r *logRepo) AppendLog(ctx context.Context, entry entity.ActionLog) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	row := []interface{}{
		entry.Timestamp.Format(time.DateTime),
		entry.CompanyID,
		entry.CompanyName,
		entry.TriggerType,
		entry.TemplateID,
		entry.Channel,
		entry.Details,
	}

	return r.sheetsClient.AppendRows(ctx, cache.RangeActionLog, [][]interface{}{row})
}

// SentTodayAlready checks if a WA message was already sent to this company today.
// Reads directly from Sheets (never cached).
func (r *logRepo) SentTodayAlready(ctx context.Context, companyID string) (bool, error) {
	rows, err := r.sheetsClient.ReadRange(ctx, cache.RangeActionLog)
	if err != nil {
		return false, fmt.Errorf("failed to read action log: %w", err)
	}

	today := time.Now().Format("2006-01-02")

	// Skip first 3 rows (indices 0-2): 2 info rows + 1 column header row
	for i, row := range rows {
		if i < 3 {
			continue
		}
		if safeString(row, 1) == companyID &&
			safeString(row, 5) == entity.ChannelWhatsApp {
			ts := safeString(row, 2)
			if len(ts) >= 10 && ts[:10] == today {
				return true, nil
			}
		}
	}

	return false, nil
}

// MessageIDExists checks if a message ID already exists in the action log (deduplication).
// Reads directly from Sheets (never cached).
func (r *logRepo) MessageIDExists(ctx context.Context, messageID string) (bool, error) {
	if messageID == "" {
		return false, nil
	}

	rows, err := r.sheetsClient.ReadRange(ctx, cache.RangeActionLog)
	if err != nil {
		return false, fmt.Errorf("failed to read action log: %w", err)
	}

	// Skip first 3 rows (indices 0-2): 2 info rows + 1 column header row
	for i, row := range rows {
		if i < 3 {
			continue
		}
		if safeString(row, 4) == messageID {
			return true, nil
		}
	}

	return false, nil
}
