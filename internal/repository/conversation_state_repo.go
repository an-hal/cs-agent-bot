package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
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
	DB           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewConversationStateRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) ConversationStateRepository {
	return &conversationStateRepo{
		DB:           db,
		queryTimeout: queryTimeout,
		tracer:       tr,
		logger:       logger,
	}
}

func (r *conversationStateRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// csColumns lists all conversation_states columns in scan order.
var csColumns = []string{
	"company_id",
	"workspace_id",
	"company_name",
	"active_flow",
	"current_stage",
	"last_message_type",
	"last_message_date",
	"response_status",
	"response_classification",
	"attempt_count",
	"cooldown_until",
	"bot_active",
	"reason_bot_paused",
	"next_scheduled_action",
	"next_scheduled_date",
	"human_owner_notified",
}

// scanConversationState scans a single row into a ConversationState struct.
func scanConversationState(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.ConversationState, error) {
	var s entity.ConversationState
	err := scanner.Scan(
		&s.CompanyID,
		&s.WorkspaceID,
		&s.CompanyName,
		&s.ActiveFlow,
		&s.CurrentStage,
		&s.LastMessageType,
		&s.LastMessageDate,
		&s.ResponseStatus,
		&s.ResponseClassification,
		&s.AttemptCount,
		&s.CooldownUntil,
		&s.BotActive,
		&s.ReasonBotPaused,
		&s.NextScheduledAction,
		&s.NextScheduledDate,
		&s.HumanOwnerNotified,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// csValues returns the column values for a ConversationState in csColumns order.
func csValues(s entity.ConversationState) []interface{} {
	return []interface{}{
		s.CompanyID,
		s.WorkspaceID,
		s.CompanyName,
		s.ActiveFlow,
		s.CurrentStage,
		s.LastMessageType,
		s.LastMessageDate,
		s.ResponseStatus,
		s.ResponseClassification,
		s.AttemptCount,
		s.CooldownUntil,
		s.BotActive,
		s.ReasonBotPaused,
		s.NextScheduledAction,
		s.NextScheduledDate,
		s.HumanOwnerNotified,
	}
}

func (r *conversationStateRepo) GetByCompanyID(ctx context.Context, companyID string) (*entity.ConversationState, error) {
	ctx, span := r.tracer.Start(ctx, "conversationState.repository.GetByCompanyID")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Select(csColumns...).
		From("conversation_states").
		Where(sq.Eq{"company_id": companyID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	s, err := scanConversationState(r.DB.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return &entity.ConversationState{
				CompanyID:      companyID,
				BotActive:      true,
				ResponseStatus: entity.ResponseStatusPending,
			}, nil
		}
		return nil, fmt.Errorf("query conversation state: %w", err)
	}

	return s, nil
}

func (r *conversationStateRepo) CreateOrUpdate(ctx context.Context, state entity.ConversationState) error {
	ctx, span := r.tracer.Start(ctx, "conversationState.repository.CreateOrUpdate")
	defer span.End()

	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	// Build ON CONFLICT SET clause using EXCLUDED.* for all columns except company_id.
	var setParts []string
	for _, col := range csColumns {
		if col == "company_id" {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
	}
	setParts = append(setParts, "updated_at = NOW()")
	upsertSuffix := fmt.Sprintf(
		"ON CONFLICT (company_id) DO UPDATE SET %s",
		strings.Join(setParts, ", "),
	)

	query, args, err := database.PSQL.
		Insert("conversation_states").
		Columns(csColumns...).
		Values(csValues(state)...).
		Suffix(upsertSuffix).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}

	if _, err = r.DB.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert conversation state: %w", err)
	}

	return nil
}

func (r *conversationStateRepo) SetBotActive(ctx context.Context, companyID string, active bool, reason string) error {
	ctx, span := r.tracer.Start(ctx, "conversationState.repository.SetBotActive")
	defer span.End()

	state, err := r.GetByCompanyID(ctx, companyID)
	if err != nil {
		return fmt.Errorf("get state for SetBotActive: %w", err)
	}

	state.BotActive = active
	state.ReasonBotPaused = &reason

	return r.CreateOrUpdate(ctx, *state)
}

func (r *conversationStateRepo) SetCooldown(ctx context.Context, companyID string, duration time.Duration) error {
	ctx, span := r.tracer.Start(ctx, "conversationState.repository.SetCooldown")
	defer span.End()

	state, err := r.GetByCompanyID(ctx, companyID)
	if err != nil {
		return fmt.Errorf("get state for SetCooldown: %w", err)
	}

	state.SetCooldown(duration)

	return r.CreateOrUpdate(ctx, *state)
}

func (r *conversationStateRepo) RecordMessage(ctx context.Context, companyID string, messageType, templateID string) error {
	ctx, span := r.tracer.Start(ctx, "conversationState.repository.RecordMessage")
	defer span.End()

	state, err := r.GetByCompanyID(ctx, companyID)
	if err != nil {
		return fmt.Errorf("get state for RecordMessage: %w", err)
	}

	state.RecordMessage(messageType, templateID)

	return r.CreateOrUpdate(ctx, *state)
}
