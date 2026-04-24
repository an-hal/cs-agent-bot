// Package manualaction implements the GUARD manual-flow overlay: the dashboard
// queue that surfaces human-composition reminders (20 flows per spec). The bot
// calls CreatePending when a manual-flow trigger fires; the human opens the
// dashboard, composes + sends via personal channels, then marks the row
// sent/skipped via MarkSent/Skip.
//
// Cross-cutting hooks (Telegram notifier, activity logger, master_data writer)
// are optional interfaces — if nil, the corresponding side effect is skipped
// with a warn log. Wiring is incremental; FE CRUD works with or without them.
package manualaction

import (
	"context"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// Optional side-effect interfaces — nil is legal.
type (
	TelegramNotifier interface {
		NotifyManualQueued(ctx context.Context, a *entity.ManualAction) error
		NotifyManualEscalation(ctx context.Context, a *entity.ManualAction) error
	}
	ActivityLogger interface {
		LogManualHumanSend(ctx context.Context, a *entity.ManualAction, notes string) error
	}
	MasterDataWriter interface {
		StampSentFlag(ctx context.Context, workspaceID, masterDataID, triggerID string) error
	}
)

// Usecase is the manual action management surface.
type Usecase interface {
	CreatePending(ctx context.Context, in CreatePendingInput) (*entity.ManualAction, error)
	Get(ctx context.Context, workspaceID, id string) (*entity.ManualAction, error)
	List(ctx context.Context, filter entity.ManualActionFilter) ([]entity.ManualAction, int64, error)
	MarkSent(ctx context.Context, workspaceID, id string, req MarkSentRequest) (*entity.ManualAction, error)
	Skip(ctx context.Context, workspaceID, id, reason string) (*entity.ManualAction, error)
	ExpirePastDue(ctx context.Context, olderThan time.Duration) (int, error)
}

type CreatePendingInput struct {
	WorkspaceID    string
	MasterDataID   string
	TriggerID      string
	FlowCategory   string
	Role           string
	AssignedToUser string
	Priority       string
	DueAt          time.Time
	SuggestedDraft string
	ContextSummary map[string]any
}

type MarkSentRequest struct {
	Channel       string `json:"channel"`
	ActualMessage string `json:"actual_message"`
	Notes         string `json:"notes"`
}

type usecase struct {
	repo       repository.ManualActionRepository
	telegram   TelegramNotifier
	activity   ActivityLogger
	masterData MasterDataWriter
	logger     zerolog.Logger
}

// New constructs a manual action usecase. telegram/activity/masterData may be nil.
func New(
	repo repository.ManualActionRepository,
	telegram TelegramNotifier,
	activity ActivityLogger,
	masterData MasterDataWriter,
	logger zerolog.Logger,
) Usecase {
	return &usecase{
		repo:       repo,
		telegram:   telegram,
		activity:   activity,
		masterData: masterData,
		logger:     logger,
	}
}

func isTerminalStatus(s string) bool {
	switch s {
	case entity.ManualActionStatusSent, entity.ManualActionStatusSkipped, entity.ManualActionStatusExpired:
		return true
	}
	return false
}

func validChannel(c string) bool {
	switch c {
	case entity.ManualActionChannelWA,
		entity.ManualActionChannelEmail,
		entity.ManualActionChannelCall,
		entity.ManualActionChannelMeeting:
		return true
	}
	return false
}

// ─── Write side: bot/cron entry point ───────────────────────────────────────

func (u *usecase) CreatePending(ctx context.Context, in CreatePendingInput) (*entity.ManualAction, error) {
	if in.WorkspaceID == "" || in.MasterDataID == "" {
		return nil, apperror.ValidationError("workspace_id and master_data_id required")
	}
	if in.TriggerID == "" || in.FlowCategory == "" || in.Role == "" {
		return nil, apperror.ValidationError("trigger_id, flow_category and role required")
	}
	if strings.TrimSpace(in.AssignedToUser) == "" {
		return nil, apperror.ValidationError("assigned_to_user required")
	}
	if in.DueAt.IsZero() {
		return nil, apperror.ValidationError("due_at required")
	}
	if in.Priority == "" {
		in.Priority = entity.ManualActionPriorityP2
	}
	if in.ContextSummary == nil {
		in.ContextSummary = map[string]any{}
	}

	out, err := u.repo.Insert(ctx, &entity.ManualAction{
		WorkspaceID:    in.WorkspaceID,
		MasterDataID:   in.MasterDataID,
		TriggerID:      in.TriggerID,
		FlowCategory:   in.FlowCategory,
		Role:           in.Role,
		AssignedToUser: strings.ToLower(in.AssignedToUser),
		SuggestedDraft: in.SuggestedDraft,
		ContextSummary: in.ContextSummary,
		Status:         entity.ManualActionStatusPending,
		Priority:       in.Priority,
		DueAt:          in.DueAt,
	})
	if err != nil {
		return nil, err
	}

	if u.telegram != nil {
		if err := u.telegram.NotifyManualQueued(ctx, out); err != nil {
			u.logger.Warn().Err(err).Str("manual_action_id", out.ID).
				Msg("telegram notify queued failed — continuing")
		}
	}
	return out, nil
}

// ─── Read side ──────────────────────────────────────────────────────────────

func (u *usecase) Get(ctx context.Context, workspaceID, id string) (*entity.ManualAction, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	m, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, apperror.NotFound("manual_action", id)
	}
	return m, nil
}

func (u *usecase) List(ctx context.Context, filter entity.ManualActionFilter) ([]entity.ManualAction, int64, error) {
	if filter.WorkspaceID == "" {
		return nil, 0, apperror.ValidationError("workspace_id required")
	}
	return u.repo.List(ctx, filter)
}

// ─── Human actions ──────────────────────────────────────────────────────────

func (u *usecase) MarkSent(ctx context.Context, workspaceID, id string, req MarkSentRequest) (*entity.ManualAction, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	if !validChannel(req.Channel) {
		return nil, apperror.ValidationError("channel must be one of wa|email|call|meeting")
	}
	if strings.TrimSpace(req.ActualMessage) == "" {
		return nil, apperror.ValidationError("actual_message required")
	}

	m, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, apperror.NotFound("manual_action", id)
	}
	if isTerminalStatus(m.Status) {
		return nil, apperror.BadRequest("manual action already in terminal status: " + m.Status)
	}

	now := time.Now().UTC()
	m.Status = entity.ManualActionStatusSent
	m.SentAt = &now
	m.SentChannel = req.Channel
	m.ActualMessage = req.ActualMessage

	out, err := u.repo.Update(ctx, m)
	if err != nil {
		return nil, err
	}

	if u.activity != nil {
		if err := u.activity.LogManualHumanSend(ctx, out, req.Notes); err != nil {
			u.logger.Warn().Err(err).Str("manual_action_id", out.ID).
				Msg("activity log failed — continuing")
		}
	}
	if u.masterData != nil {
		if err := u.masterData.StampSentFlag(ctx, out.WorkspaceID, out.MasterDataID, out.TriggerID); err != nil {
			u.logger.Warn().Err(err).Str("manual_action_id", out.ID).
				Msg("master_data stamp failed — continuing")
		}
	}
	return out, nil
}

func (u *usecase) Skip(ctx context.Context, workspaceID, id, reason string) (*entity.ManualAction, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	if len(strings.TrimSpace(reason)) < 5 {
		return nil, apperror.ValidationError("reason must be at least 5 characters")
	}

	m, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, apperror.NotFound("manual_action", id)
	}
	if isTerminalStatus(m.Status) {
		return nil, apperror.BadRequest("manual action already in terminal status: " + m.Status)
	}

	m.Status = entity.ManualActionStatusSkipped
	m.SkippedReason = strings.TrimSpace(reason)
	return u.repo.Update(ctx, m)
}

// ─── Background job ─────────────────────────────────────────────────────────

func (u *usecase) ExpirePastDue(ctx context.Context, olderThan time.Duration) (int, error) {
	if olderThan <= 0 {
		olderThan = 48 * time.Hour
	}
	cutoff := time.Now().UTC().Add(-olderThan)
	rows, err := u.repo.ListPastDue(ctx, cutoff, 500)
	if err != nil {
		return 0, err
	}
	n := 0
	for i := range rows {
		rows[i].Status = entity.ManualActionStatusExpired
		if _, err := u.repo.Update(ctx, &rows[i]); err != nil {
			u.logger.Warn().Err(err).Str("manual_action_id", rows[i].ID).
				Msg("expire past-due failed — continuing")
			continue
		}
		if u.telegram != nil {
			_ = u.telegram.NotifyManualEscalation(ctx, &rows[i])
		}
		n++
	}
	return n, nil
}
