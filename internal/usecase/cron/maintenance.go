package cron

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// MaintenanceRunner runs daily maintenance jobs: days_to_expiry recalc and overdue marking.
type MaintenanceRunner struct {
	masterDataRepo repository.MasterDataRepository
	logger         zerolog.Logger
}

// NewMaintenanceRunner constructs a MaintenanceRunner.
func NewMaintenanceRunner(masterDataRepo repository.MasterDataRepository, logger zerolog.Logger) *MaintenanceRunner {
	return &MaintenanceRunner{masterDataRepo: masterDataRepo, logger: logger}
}

// UpdateDaysToExpiry recalculates days_to_expiry for all CLIENT records in a workspace.
// Runs daily at 00:05 WIB.
func (m *MaintenanceRunner) UpdateDaysToExpiry(ctx context.Context, workspaceID string) error {
	n, err := m.masterDataRepo.BulkUpdateDaysToExpiry(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("UpdateDaysToExpiry: %w", err)
	}
	m.logger.Info().Str("workspace_id", workspaceID).Int64("updated", n).Msg("days_to_expiry recalculated")
	return nil
}

// CheckOverduePayments marks pending payments as overdue when contract_end has passed.
// Runs every 4 hours (no working-day restriction — P6 urgency).
func (m *MaintenanceRunner) CheckOverduePayments(ctx context.Context, workspaceID string) error {
	n, err := m.masterDataRepo.BulkMarkOverdue(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("CheckOverduePayments: %w", err)
	}
	m.logger.Info().Str("workspace_id", workspaceID).Int64("marked", n).Msg("overdue payments marked")
	return nil
}
