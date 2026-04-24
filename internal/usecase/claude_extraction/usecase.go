// Package claudeextraction owns Claude-driven field extraction + BANTS
// scoring for Fireflies transcripts and other sources. The actual Claude API
// call is abstracted behind the Extractor interface — real client lives
// elsewhere; here we orchestrate persistence and state transitions.
package claudeextraction

import (
	"context"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// Client is the external Claude API boundary. A stub impl that returns an
// empty result is legal (returns nil result + nil error; usecase marks
// extraction as `succeeded` with no data).
type Client interface {
	Extract(ctx context.Context, transcriptText string, hints map[string]any) (*Result, error)
}

// Result is the normalized output from the Claude client.
type Result struct {
	Fields              map[string]any
	Model               string
	PromptTemplate      string
	BANTSBudget         *int
	BANTSAuthority      *int
	BANTSNeed           *int
	BANTSTiming         *int
	BANTSSentiment      *int
	BANTSScore          *float64
	BANTSClassification string
	BuyingIntent        string
	CoachingNotes       string
	PromptTokens        int
	CompletionTokens    int
}

type Usecase interface {
	// Start enqueues an extraction attempt for the given source (idempotent
	// per source: marks prior attempts as superseded). Returns the pending
	// row; the actual extraction happens async via Run.
	Start(ctx context.Context, req StartRequest) (*entity.ClaudeExtraction, error)
	// Run executes a pending extraction synchronously — suitable for cron or
	// request-scoped processing. Updates status + result in place.
	Run(ctx context.Context, workspaceID, id, transcriptText string, hints map[string]any) (*entity.ClaudeExtraction, error)
	Get(ctx context.Context, workspaceID, id string) (*entity.ClaudeExtraction, error)
	ListForMasterData(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ClaudeExtraction, error)
}

type StartRequest struct {
	WorkspaceID  string
	SourceType   string
	SourceID     string
	MasterDataID string
}

type usecase struct {
	repo   repository.ClaudeExtractionRepository
	client Client
	logger zerolog.Logger
}

func New(repo repository.ClaudeExtractionRepository, client Client, logger zerolog.Logger) Usecase {
	return &usecase{repo: repo, client: client, logger: logger}
}

func (u *usecase) Start(ctx context.Context, req StartRequest) (*entity.ClaudeExtraction, error) {
	if req.WorkspaceID == "" || req.SourceType == "" || req.SourceID == "" {
		return nil, apperror.ValidationError("workspace_id, source_type and source_id required")
	}

	out, err := u.repo.Insert(ctx, &entity.ClaudeExtraction{
		WorkspaceID:  req.WorkspaceID,
		SourceType:   req.SourceType,
		SourceID:     req.SourceID,
		MasterDataID: req.MasterDataID,
		Status:       entity.ClaudeExtractionStatusPending,
	})
	if err != nil {
		return nil, err
	}
	// Mark prior attempts for the same source as superseded.
	if err := u.repo.MarkSuperseded(ctx, out.WorkspaceID, out.SourceType, out.SourceID, out.ID); err != nil {
		u.logger.Warn().Err(err).Str("extraction_id", out.ID).
			Msg("mark superseded failed (non-fatal)")
	}
	return out, nil
}

func (u *usecase) Run(ctx context.Context, workspaceID, id, transcriptText string, hints map[string]any) (*entity.ClaudeExtraction, error) {
	row, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, apperror.NotFound("claude_extraction", id)
	}
	if row.Status != entity.ClaudeExtractionStatusPending && row.Status != entity.ClaudeExtractionStatusFailed {
		return nil, apperror.BadRequest("extraction is not pending/failed (status=" + row.Status + ")")
	}

	if u.client == nil {
		// No client wired — mark failed with a clear message; row remains
		// retryable when Claude integration is configured.
		row.Status = entity.ClaudeExtractionStatusFailed
		row.ErrorMessage = "Claude client not configured"
		if err := u.repo.UpdateResult(ctx, row); err != nil {
			return nil, err
		}
		return row, nil
	}

	start := time.Now()
	res, err := u.client.Extract(ctx, transcriptText, hints)
	latency := time.Since(start).Milliseconds()

	row.LatencyMS = int(latency)
	if err != nil {
		row.Status = entity.ClaudeExtractionStatusFailed
		row.ErrorMessage = err.Error()
		_ = u.repo.UpdateResult(ctx, row)
		return row, err
	}
	if res == nil {
		// Client returned no data but no error — treat as succeeded-empty
		// so we don't retry forever.
		row.Status = entity.ClaudeExtractionStatusSucceeded
		_ = u.repo.UpdateResult(ctx, row)
		return row, nil
	}

	row.ExtractedFields = res.Fields
	row.ExtractionModel = res.Model
	row.ExtractionPrompt = res.PromptTemplate
	row.BANTSBudget = res.BANTSBudget
	row.BANTSAuthority = res.BANTSAuthority
	row.BANTSNeed = res.BANTSNeed
	row.BANTSTiming = res.BANTSTiming
	row.BANTSSentiment = res.BANTSSentiment
	row.BANTSScore = res.BANTSScore
	row.BANTSClassification = strings.ToLower(res.BANTSClassification)
	row.BuyingIntent = strings.ToLower(res.BuyingIntent)
	row.CoachingNotes = res.CoachingNotes
	row.PromptTokens = res.PromptTokens
	row.CompletionTokens = res.CompletionTokens
	row.Status = entity.ClaudeExtractionStatusSucceeded
	row.ErrorMessage = ""

	if err := u.repo.UpdateResult(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

func (u *usecase) Get(ctx context.Context, workspaceID, id string) (*entity.ClaudeExtraction, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	out, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, apperror.NotFound("claude_extraction", id)
	}
	return out, nil
}

func (u *usecase) ListForMasterData(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ClaudeExtraction, error) {
	if workspaceID == "" || masterDataID == "" {
		return nil, apperror.ValidationError("workspace_id and master_data_id required")
	}
	return u.repo.ListForMasterData(ctx, workspaceID, masterDataID, limit)
}
