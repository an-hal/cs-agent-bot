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

type ConversationStateRepository interface {
	GetByCompanyID(ctx context.Context, companyID string) (*entity.ConversationState, error)
	CreateOrUpdate(ctx context.Context, state entity.ConversationState) error
	SetBotActive(ctx context.Context, companyID string, active bool, reason string) error
	SetCooldown(ctx context.Context, companyID string, duration time.Duration) error
	RecordMessage(ctx context.Context, companyID string, messageType, templateID string) error
}

type conversationStateRepo struct {
	sheetsClient *sheets.SheetsClient
	cache        cache.SheetCache
	logger       zerolog.Logger
}

func NewConversationStateRepo(
	sheetsClient *sheets.SheetsClient,
	cache cache.SheetCache,
	logger zerolog.Logger,
) ConversationStateRepository {
	return &conversationStateRepo{
		sheetsClient: sheetsClient,
		cache:        cache,
		logger:       logger,
	}
}

func (r *conversationStateRepo) GetByCompanyID(ctx context.Context, companyID string) (*entity.ConversationState, error) {
	rows, err := r.cache.Get(ctx, cache.KeyConversationState, cache.RangeConversationState, cache.TTLConversationState)
	if err != nil {
		return nil, err
	}

	for i, row := range rows {
		if i < 3 { // Skip header rows
			continue
		}
		if safeString(row, 0) == companyID {
			return parseConversationStateRow(row)
		}
	}

	// If not found, return default state
	return &entity.ConversationState{
		CompanyID:              companyID,
		BotActive:              true,
		ResponseStatus:         entity.ResponseStatusPending,
		AttemptCount:           0,
		HumanOwnerNotified:     false,
		NextScheduledDate:      time.Time{},
		LastMessageDate:        time.Time{},
		CooldownUntil:          time.Time{},
	}, nil
}

func (r *conversationStateRepo) CreateOrUpdate(ctx context.Context, state entity.ConversationState) error {
	rows, err := r.cache.Get(ctx, cache.KeyConversationState, cache.RangeConversationState, cache.TTLConversationState)
	if err != nil {
		return err
	}

	// Check if exists
	for i, row := range rows {
		if i < 3 {
			continue
		}
		if safeString(row, 0) == state.CompanyID {
			// Update existing row
			stateRow := conversationStateToRow(state)
			sheetRange := fmt.Sprintf("%s!A%d", cache.RangeConversationState, i+1)
			if err := r.sheetsClient.WriteRange(ctx, sheetRange, [][]interface{}{stateRow}); err != nil {
				return err
			}
			return r.cache.Invalidate(ctx, cache.KeyConversationState)
		}
	}

	// Create new row
	stateRow := conversationStateToRow(state)
	if err := r.sheetsClient.AppendRows(ctx, cache.RangeConversationState, [][]interface{}{stateRow}); err != nil {
		return err
	}
	return r.cache.Invalidate(ctx, cache.KeyConversationState)
}

func (r *conversationStateRepo) SetBotActive(ctx context.Context, companyID string, active bool, reason string) error {
	state, err := r.GetByCompanyID(ctx, companyID)
	if err != nil {
		return err
	}
	state.BotActive = active
	state.ReasonBotPaused = reason
	return r.CreateOrUpdate(ctx, *state)
}

func (r *conversationStateRepo) SetCooldown(ctx context.Context, companyID string, duration time.Duration) error {
	state, err := r.GetByCompanyID(ctx, companyID)
	if err != nil {
		return err
	}
	state.SetCooldown(duration)
	return r.CreateOrUpdate(ctx, *state)
}

func (r *conversationStateRepo) RecordMessage(ctx context.Context, companyID string, messageType, templateID string) error {
	state, err := r.GetByCompanyID(ctx, companyID)
	if err != nil {
		return err
	}
	state.RecordMessage(messageType, templateID)
	return r.CreateOrUpdate(ctx, *state)
}

func parseConversationStateRow(row []interface{}) (*entity.ConversationState, error) {
	if len(row) < 15 {
		return nil, fmt.Errorf("conversation state row too short: %d columns", len(row))
	}

	return &entity.ConversationState{
		CompanyID:              safeString(row, 0),
		CompanyName:            safeString(row, 1),
		ActiveFlow:             safeString(row, 2),
		CurrentStage:           safeString(row, 3),
		LastMessageType:        safeString(row, 4),
		LastMessageDate:        safeDate(row, 5),
		ResponseStatus:         safeString(row, 6),
		ResponseClassification: safeString(row, 7),
		AttemptCount:           safeInt(row, 8),
		CooldownUntil:          safeDate(row, 9),
		BotActive:              safeBool(row, 10),
		ReasonBotPaused:        safeString(row, 11),
		NextScheduledAction:    safeString(row, 12),
		NextScheduledDate:      safeDate(row, 13),
		HumanOwnerNotified:     safeBool(row, 14),
	}, nil
}

func conversationStateToRow(cs entity.ConversationState) []interface{} {
	lastMsgDate := ""
	if !cs.LastMessageDate.IsZero() {
		lastMsgDate = cs.LastMessageDate.Format("2006-01-02")
	}

	cooldownDate := ""
	if !cs.CooldownUntil.IsZero() {
		cooldownDate = cs.CooldownUntil.Format("2006-01-02")
	}

	nextScheduledDate := ""
	if !cs.NextScheduledDate.IsZero() {
		nextScheduledDate = cs.NextScheduledDate.Format("2006-01-02")
	}

	return []interface{}{
		cs.CompanyID,
		cs.CompanyName,
		cs.ActiveFlow,
		cs.CurrentStage,
		cs.LastMessageType,
		lastMsgDate,
		cs.ResponseStatus,
		cs.ResponseClassification,
		cs.AttemptCount,
		cooldownDate,
		cs.BotActive,
		cs.ReasonBotPaused,
		cs.NextScheduledAction,
		nextScheduledDate,
		cs.HumanOwnerNotified,
	}
}
