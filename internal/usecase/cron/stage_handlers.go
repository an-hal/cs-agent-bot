package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// StageTransitionHandler processes pipeline stage transitions triggered by
// automation rules (SDR→BD handoff, BD→AE handoff, dormant recycle, renewal reset).
type StageTransitionHandler interface {
	HandleTransition(ctx context.Context, triggerID string, md *entity.MasterData) error
}

type stageHandler struct {
	masterDataRepo repository.MasterDataRepository
	actionLogRepo  repository.ActionLogWorkflowRepository
	logger         zerolog.Logger
}

// NewStageHandler creates a StageTransitionHandler.
func NewStageHandler(masterDataRepo repository.MasterDataRepository, actionLogRepo repository.ActionLogWorkflowRepository, logger zerolog.Logger) StageTransitionHandler {
	return &stageHandler{masterDataRepo: masterDataRepo, actionLogRepo: actionLogRepo, logger: logger}
}

func (h *stageHandler) HandleTransition(ctx context.Context, triggerID string, md *entity.MasterData) error {
	switch triggerID {
	case "SDR_QUALIFY_HANDOFF":
		return h.sdrQualifyHandoff(ctx, md)
	case "BD_PAYMENT_HANDOFF":
		return h.bdPaymentHandoff(ctx, md)
	case "BD_DORMANT_TO_SDR":
		return h.bdDormantToSDR(ctx, md)
	case "RENEWAL_CYCLE_RESET":
		return h.renewalCycleReset(ctx, md)
	default:
		return fmt.Errorf("unknown stage transition: %s", triggerID)
	}
}

func (h *stageHandler) sdrQualifyHandoff(ctx context.Context, md *entity.MasterData) error {
	_, _, err := h.masterDataRepo.Transition(ctx, md.WorkspaceID, md.ID, entity.StageProspect, repository.MasterDataPatch{
		SequenceStatus: strPtr(entity.SeqStatusActive),
	}, map[string]any{
		"qualified_at": time.Now().Format(time.RFC3339),
		"qualified_by": md.OwnerName,
		"wa_h0_sent":   false, "wa_h1_sent": false, "wa_h3_sent": false,
		"wa_h7_sent": false, "wa_h8_sent": false, "wa_h14_sent": false,
	})
	if err != nil {
		return fmt.Errorf("SDR_QUALIFY_HANDOFF transition: %w", err)
	}
	h.logger.Info().Str("company_id", md.CompanyID).Msg("SDR→BD handoff complete")
	return nil
}

func (h *stageHandler) bdPaymentHandoff(ctx context.Context, md *entity.MasterData) error {
	now := time.Now()
	months := md.ContractMonths
	if months == 0 {
		months = 12
	}
	end := now.AddDate(0, months, 0)
	daysToExpiry := int(end.Sub(now).Hours() / 24)

	_, _, err := h.masterDataRepo.Transition(ctx, md.WorkspaceID, md.ID, entity.StageClient, repository.MasterDataPatch{
		BotActive:      boolPtr(true),
		SequenceStatus: strPtr(entity.SeqStatusActive),
		ContractStart:  &now,
		ContractEnd:    &end,
		ContractMonths: &months,
	}, map[string]any{
		"closing_status": "CLOSED_WON",
		"d0_sent": false, "d2_sent": false, "d4_sent": false,
		"d7_sent": false, "d10_sent": false, "d12_sent": false,
		"days_to_expiry": daysToExpiry,
	})
	if err != nil {
		return fmt.Errorf("BD_PAYMENT_HANDOFF transition: %w", err)
	}
	h.logger.Info().Str("company_id", md.CompanyID).Msg("BD→AE handoff complete")
	return nil
}

func (h *stageHandler) bdDormantToSDR(ctx context.Context, md *entity.MasterData) error {
	_, _, err := h.masterDataRepo.Transition(ctx, md.WorkspaceID, md.ID, entity.StageLead, repository.MasterDataPatch{
		BotActive:      boolPtr(true),
		SequenceStatus: strPtr(entity.SeqStatusActive),
	}, map[string]any{
		"lead_segment": "RECYCLED",
		"d0_sent": false, "d2_sent": false, "d4_sent": false,
		"d7_sent": false, "d10_sent": false, "d12_sent": false,
		"d14_sent": false, "d21_sent": false,
		"nurture_d45_sent": false, "nurture_d75_sent": false, "nurture_d90_sent": false,
	})
	if err != nil {
		return fmt.Errorf("BD_DORMANT_TO_SDR transition: %w", err)
	}
	h.logger.Info().Str("company_id", md.CompanyID).Msg("BD dormant→SDR recycle complete")
	return nil
}

func (h *stageHandler) renewalCycleReset(ctx context.Context, md *entity.MasterData) error {
	now := time.Now()
	months := md.ContractMonths
	if months == 0 {
		months = 12
	}
	end := now.AddDate(0, months, 0)

	// Reset all AE-phase flags for new cycle.
	updates := map[string]any{
		"renewed":       false,
		"sequence_status": entity.SeqStatusActive,
		"onboarding_sent": false,
		"checkin_sent":   false,
		"nps_sent":       false,
		"renewal_h30_sent": false,
		"renewal_h14_sent": false,
		"renewal_h7_sent":  false,
		"renewal_h0_sent":  false,
	}

	// Update core fields via Patch.
	_, err := h.masterDataRepo.Patch(ctx, md.WorkspaceID, md.ID, repository.MasterDataPatch{
		BotActive:      boolPtr(true),
		ContractStart:  &now,
		ContractEnd:    &end,
		ContractMonths: &months,
	})
	if err != nil {
		return fmt.Errorf("RENEWAL_CYCLE_RESET patch: %w", err)
	}

	if err := h.masterDataRepo.MergeCustomFields(ctx, md.WorkspaceID, md.ID, updates); err != nil {
		return fmt.Errorf("RENEWAL_CYCLE_RESET custom: %w", err)
	}

	h.logger.Info().Str("company_id", md.CompanyID).Msg("renewal cycle reset complete")
	return nil
}
