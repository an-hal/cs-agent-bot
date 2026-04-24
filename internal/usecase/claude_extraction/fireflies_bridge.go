package claudeextraction

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// FirefliesBridge implements fireflies.Extractor by translating a Fireflies
// transcript into a Claude extraction attempt. Created separately so the
// fireflies package does not need to depend on this package (keeps the
// dependency graph flat).
type FirefliesBridge struct {
	extractionUC Usecase
	firefliesRepo repository.FirefliesTranscriptRepository
	logger        zerolog.Logger
}

// NewFirefliesBridge wires Fireflies transcripts into the Claude extraction
// pipeline. Passing nil usecase makes ExtractFromFireflies a no-op so this is
// safe to wire whether or not the Claude API key is present.
func NewFirefliesBridge(extractionUC Usecase, firefliesRepo repository.FirefliesTranscriptRepository, logger zerolog.Logger) *FirefliesBridge {
	return &FirefliesBridge{extractionUC: extractionUC, firefliesRepo: firefliesRepo, logger: logger}
}

// ExtractFromFireflies reads the transcript, starts a Claude extraction row,
// runs it, and updates the fireflies_transcripts row's extraction_status.
// Errors are logged but not returned fatally — the caller (fireflies.Usecase)
// treats this as best-effort background work.
func (b *FirefliesBridge) ExtractFromFireflies(ctx context.Context, workspaceID, transcriptID string) error {
	if b == nil || b.extractionUC == nil || b.firefliesRepo == nil {
		return nil
	}
	t, err := b.firefliesRepo.GetByID(ctx, workspaceID, transcriptID)
	if err != nil {
		return fmt.Errorf("load fireflies transcript: %w", err)
	}
	if t == nil {
		return fmt.Errorf("fireflies transcript %s not found", transcriptID)
	}
	// Mark as running so FE sees progress.
	if err := b.firefliesRepo.UpdateExtraction(ctx, workspaceID, transcriptID, entity.FirefliesStatusRunning, "", ""); err != nil {
		b.logger.Warn().Err(err).Str("transcript_id", transcriptID).Msg("mark running failed")
	}

	start, err := b.extractionUC.Start(ctx, StartRequest{
		WorkspaceID: workspaceID,
		SourceType:  entity.ClaudeSourceFireflies,
		SourceID:    transcriptID,
	})
	if err != nil {
		_ = b.firefliesRepo.UpdateExtraction(ctx, workspaceID, transcriptID, entity.FirefliesStatusFailed, err.Error(), "")
		return err
	}

	result, err := b.extractionUC.Run(ctx, workspaceID, start.ID, t.TranscriptText, map[string]any{
		"meeting_title":    t.MeetingTitle,
		"host_email":       t.HostEmail,
		"participants":     t.Participants,
		"duration_seconds": t.DurationSeconds,
	})
	if err != nil {
		_ = b.firefliesRepo.UpdateExtraction(ctx, workspaceID, transcriptID, entity.FirefliesStatusFailed, err.Error(), "")
		return err
	}

	status := entity.FirefliesStatusSucceeded
	errMsg := ""
	if result.Status == entity.ClaudeExtractionStatusFailed {
		status = entity.FirefliesStatusFailed
		errMsg = result.ErrorMessage
	}
	return b.firefliesRepo.UpdateExtraction(ctx, workspaceID, transcriptID, status, errMsg, result.MasterDataID)
}
