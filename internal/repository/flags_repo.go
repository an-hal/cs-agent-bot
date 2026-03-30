package repository

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database/sheets"
	"github.com/Sejutacita/cs-agent-bot/internal/service/cache"
	"github.com/rs/zerolog"
)

type FlagsRepository interface {
	GetByCompanyID(ctx context.Context, companyID string) (*entity.ClientFlags, error)
	UpdateFlags(ctx context.Context, companyID string, flags entity.ClientFlags) error
	SetBotActive(ctx context.Context, companyID string, active bool) error
	ResetCycleFlags(ctx context.Context, companyID string) error
}

type flagsRepo struct {
	sheetsClient *sheets.SheetsClient
	cache        cache.SheetCache
	logger       zerolog.Logger
}

func NewFlagsRepo(sheetsClient *sheets.SheetsClient, cache cache.SheetCache, logger zerolog.Logger) FlagsRepository {
	return &flagsRepo{
		sheetsClient: sheetsClient,
		cache:        cache,
		logger:       logger,
	}
}

func (r *flagsRepo) GetByCompanyID(ctx context.Context, companyID string) (*entity.ClientFlags, error) {
	rows, err := r.cache.Get(ctx, cache.KeyConversationState, cache.RangeConversationState, cache.TTLConversationState)
	if err != nil {
		return nil, err
	}

	// Skip first 3 rows (indices 0-2): 2 info rows + 1 column header row
	for i, row := range rows {
		if i < 3 {
			continue
		}
		if safeString(row, 0) == companyID {
			return parseFlagsRow(row)
		}
	}

	return nil, fmt.Errorf("flags not found for company: %s", companyID)
}

func (r *flagsRepo) UpdateFlags(ctx context.Context, companyID string, flags entity.ClientFlags) error {
	rows, err := r.cache.Get(ctx, cache.KeyConversationState, cache.RangeConversationState, cache.TTLConversationState)
	if err != nil {
		return err
	}

	for i, row := range rows {
		if i < 3 {
			continue
		}
		if safeString(row, 0) == companyID {
			flagRow := flagsToRow(flags)
			// Spreadsheet row: 3 header rows + data index + 1 (1-indexed) = i + 1
			sheetRange := fmt.Sprintf("%s!A%d", cache.RangeConversationState, i+1)
			if err := r.sheetsClient.WriteRange(ctx, sheetRange, [][]interface{}{flagRow}); err != nil {
				return err
			}
			return r.cache.Invalidate(ctx, cache.KeyConversationState)
		}
	}

	return fmt.Errorf("flags not found for company: %s", companyID)
}

func (r *flagsRepo) SetBotActive(ctx context.Context, companyID string, active bool) error {
	flags, err := r.GetByCompanyID(ctx, companyID)
	if err != nil {
		return err
	}

	// bot_active is stored in Sheet 4 (Conversation State) at column 10 (1-indexed: K)
	// Find the row and update the bot_active column
	rows, err := r.cache.Get(ctx, cache.KeyConversationState, cache.RangeConversationState, cache.TTLConversationState)
	if err != nil {
		return err
	}

	for i, row := range rows {
		if i < 3 {
			continue
		}
		if safeString(row, 0) == companyID {
			_ = flags // used above to verify existence
			// Bot_Active is at column 10 (index 10, which is column K in 1-indexed)
			cellRange := fmt.Sprintf("%s!K%d", cache.RangeConversationState, i+1)
			if err := r.sheetsClient.UpdateCell(ctx, cellRange, active); err != nil {
				return err
			}
			return r.cache.Invalidate(ctx, cache.KeyConversationState)
		}
	}

	return fmt.Errorf("flags not found for company: %s", companyID)
}

// ResetCycleFlags resets all renewal, check-in, NPS, referral, and risk flags.
// Cross-sell flags are NOT reset (they persist across cycles).
func (r *flagsRepo) ResetCycleFlags(ctx context.Context, companyID string) error {
	flags, err := r.GetByCompanyID(ctx, companyID)
	if err != nil {
		return err
	}

	// Reset renewal flags
	flags.Ren60Sent = false
	flags.Ren45Sent = false
	flags.Ren30Sent = false
	flags.Ren15Sent = false
	flags.Ren0Sent = false

	// Reset check-in flags
	flags.CheckinA1FormSent = false
	flags.CheckinA1CallSent = false
	flags.CheckinA2FormSent = false
	flags.CheckinA2CallSent = false
	flags.CheckinB1FormSent = false
	flags.CheckinB1CallSent = false
	flags.CheckinB2FormSent = false
	flags.CheckinB2CallSent = false
	flags.CheckinReplied = false

	// Reset NPS + referral flags
	flags.NPS1Sent = false
	flags.NPS2Sent = false
	flags.NPS3Sent = false
	flags.NPSReplied = false
	flags.ReferralSentThisCycle = false

	// Reset risk flags
	flags.LowUsageMsgSent = false
	flags.LowNPSMsgSent = false

	// NOTE: Cross-sell flags (CSH*, CSLT*) are NOT reset

	return r.UpdateFlags(ctx, companyID, *flags)
}

func parseFlagsRow(row []interface{}) (*entity.ClientFlags, error) {
	if len(row) < 2 {
		return nil, fmt.Errorf("flags row too short: %d columns", len(row))
	}

	return &entity.ClientFlags{
		CompanyID:         safeString(row, 0),
		Ren60Sent:         safeBool(row, 1),
		Ren45Sent:         safeBool(row, 2),
		Ren30Sent:         safeBool(row, 3),
		Ren15Sent:         safeBool(row, 4),
		Ren0Sent:          safeBool(row, 5),
		CheckinA1FormSent: safeBool(row, 6),
		CheckinA1CallSent: safeBool(row, 7),
		CheckinA2FormSent: safeBool(row, 8),
		CheckinA2CallSent: safeBool(row, 9),
		CheckinB1FormSent: safeBool(row, 10),
		CheckinB1CallSent: safeBool(row, 11),
		CheckinB2FormSent: safeBool(row, 12),
		CheckinB2CallSent: safeBool(row, 13),
		CheckinReplied:    safeBool(row, 14),
		NPS1Sent:          safeBool(row, 15),
		NPS2Sent:          safeBool(row, 16),
		NPS3Sent:          safeBool(row, 17),
		NPSReplied:        safeBool(row, 18),
		ReferralSentThisCycle: safeBool(row, 19),
		LowUsageMsgSent:   safeBool(row, 20),
		LowNPSMsgSent:     safeBool(row, 21),
		CSH7:              safeBool(row, 22),
		CSH14:             safeBool(row, 23),
		CSH21:             safeBool(row, 24),
		CSH30:             safeBool(row, 25),
		CSH45:             safeBool(row, 26),
		CSH60:             safeBool(row, 27),
		CSH75:             safeBool(row, 28),
		CSH90:             safeBool(row, 29),
		CSLT1:             safeBool(row, 30),
		CSLT2:             safeBool(row, 31),
		CSLT3:             safeBool(row, 32),
		FeatureUpdateSent: safeBool(row, 33),
	}, nil
}

func flagsToRow(f entity.ClientFlags) []interface{} {
	return []interface{}{
		f.CompanyID,
		f.Ren60Sent, f.Ren45Sent, f.Ren30Sent, f.Ren15Sent, f.Ren0Sent,
		f.CheckinA1FormSent, f.CheckinA1CallSent, f.CheckinA2FormSent, f.CheckinA2CallSent,
		f.CheckinB1FormSent, f.CheckinB1CallSent, f.CheckinB2FormSent, f.CheckinB2CallSent,
		f.CheckinReplied,
		f.NPS1Sent, f.NPS2Sent, f.NPS3Sent, f.NPSReplied, f.ReferralSentThisCycle,
		f.LowUsageMsgSent, f.LowNPSMsgSent,
		f.CSH7, f.CSH14, f.CSH21, f.CSH30, f.CSH45, f.CSH60, f.CSH75, f.CSH90,
		f.CSLT1, f.CSLT2, f.CSLT3,
		f.FeatureUpdateSent,
	}
}
