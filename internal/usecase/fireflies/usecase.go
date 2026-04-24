// Package fireflies handles inbound Fireflies transcript webhooks and owns
// the lifecycle of stored transcripts (including triggering Claude extraction
// asynchronously via an injected Extractor).
package fireflies

import (
	"context"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// Extractor is the optional hook that kicks off a Claude extraction for a
// newly ingested transcript. Nil is legal — the transcript is stored and
// extraction is deferred to a future batch run.
type Extractor interface {
	ExtractFromFireflies(ctx context.Context, workspaceID, transcriptID string) error
}

type Usecase interface {
	IngestWebhook(ctx context.Context, req IngestWebhookRequest) (*entity.FirefliesTranscript, error)
	Get(ctx context.Context, workspaceID, id string) (*entity.FirefliesTranscript, error)
	List(ctx context.Context, workspaceID, status string, limit, offset int) ([]entity.FirefliesTranscript, int64, error)
}

// IngestWebhookRequest is what the Fireflies webhook delivers (normalized).
// `RawPayload` preserves the full body for replay if processing changes later.
type IngestWebhookRequest struct {
	WorkspaceID     string         `json:"workspace_id"`
	FirefliesID     string         `json:"fireflies_id"`
	MeetingTitle    string         `json:"meeting_title"`
	MeetingDateISO  string         `json:"meeting_date"`
	DurationSeconds int            `json:"duration_seconds"`
	HostEmail       string         `json:"host_email"`
	Participants    []string       `json:"participants"`
	TranscriptText  string         `json:"transcript_text"`
	RawPayload      map[string]any `json:"raw_payload"`
}

type usecase struct {
	repo      repository.FirefliesTranscriptRepository
	extractor Extractor
	logger    zerolog.Logger
}

func New(repo repository.FirefliesTranscriptRepository, extractor Extractor, logger zerolog.Logger) Usecase {
	return &usecase{repo: repo, extractor: extractor, logger: logger}
}

func (u *usecase) IngestWebhook(ctx context.Context, req IngestWebhookRequest) (*entity.FirefliesTranscript, error) {
	if req.WorkspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if strings.TrimSpace(req.FirefliesID) == "" {
		return nil, apperror.ValidationError("fireflies_id required")
	}

	// Idempotency — if this Fireflies ID is already stored, return the
	// existing row. We never overwrite since the transcript text is the
	// source of truth once ingested.
	existing, err := u.repo.GetByFirefliesID(ctx, req.WorkspaceID, req.FirefliesID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	t := &entity.FirefliesTranscript{
		WorkspaceID:     req.WorkspaceID,
		FirefliesID:     req.FirefliesID,
		MeetingTitle:    req.MeetingTitle,
		DurationSeconds: req.DurationSeconds,
		HostEmail:       strings.ToLower(req.HostEmail),
		Participants:    req.Participants,
		TranscriptText:  req.TranscriptText,
		RawPayload:      req.RawPayload,
	}
	if req.MeetingDateISO != "" {
		if parsed, ok := parseTime(req.MeetingDateISO); ok {
			t.MeetingDate = &parsed
		}
	}

	out, err := u.repo.Insert(ctx, t)
	if err != nil {
		return nil, err
	}

	// Kick off extraction asynchronously if wired. Failure is non-fatal —
	// the transcript is already persisted and can be re-processed later.
	if u.extractor != nil {
		go func(wsID, id string) {
			if err := u.extractor.ExtractFromFireflies(context.Background(), wsID, id); err != nil {
				u.logger.Warn().Err(err).Str("transcript_id", id).
					Msg("fireflies extraction trigger failed (non-fatal)")
			}
		}(out.WorkspaceID, out.ID)
	}
	return out, nil
}

func (u *usecase) Get(ctx context.Context, workspaceID, id string) (*entity.FirefliesTranscript, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	out, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, apperror.NotFound("fireflies_transcript", id)
	}
	return out, nil
}

func (u *usecase) List(ctx context.Context, workspaceID, status string, limit, offset int) ([]entity.FirefliesTranscript, int64, error) {
	if workspaceID == "" {
		return nil, 0, apperror.ValidationError("workspace_id required")
	}
	return u.repo.List(ctx, workspaceID, status, limit, offset)
}
