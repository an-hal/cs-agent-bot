// Package coaching implements the BD peer-review + manager-review workflow
// (feature 00-shared/11 BD coaching pipeline). Scores are 1–5; overall is
// computed as a simple average of non-nil sub-scores.
package coaching

import (
	"context"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

type Usecase interface {
	Create(ctx context.Context, req CreateRequest) (*entity.CoachingSession, error)
	Get(ctx context.Context, workspaceID, id string) (*entity.CoachingSession, error)
	List(ctx context.Context, filter entity.CoachingSessionFilter) ([]entity.CoachingSession, int64, error)
	Update(ctx context.Context, req UpdateRequest) (*entity.CoachingSession, error)
	Submit(ctx context.Context, workspaceID, id, actor string) (*entity.CoachingSession, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

type CreateRequest struct {
	WorkspaceID        string `json:"workspace_id"`
	BDEmail            string `json:"bd_email"`
	CoachEmail         string `json:"coach_email"`
	MasterDataID       string `json:"master_data_id"`
	ClaudeExtractionID string `json:"claude_extraction_id"`
	SessionType        string `json:"session_type"`
}

type UpdateRequest struct {
	WorkspaceID          string   `json:"workspace_id"`
	ID                   string   `json:"id"`
	BANTSClarityScore    *int     `json:"bants_clarity_score"`
	DiscoveryDepthScore  *int     `json:"discovery_depth_score"`
	ToneFitScore         *int     `json:"tone_fit_score"`
	NextStepClarityScore *int     `json:"next_step_clarity_score"`
	Strengths            string   `json:"strengths"`
	Improvements         string   `json:"improvements"`
	ActionItems          string   `json:"action_items"`
}

type usecase struct {
	repo repository.CoachingSessionRepository
}

func New(repo repository.CoachingSessionRepository) Usecase {
	return &usecase{repo: repo}
}

func validSessionType(t string) bool {
	switch t {
	case "", entity.CoachingTypePeerReview, entity.CoachingTypeSelfReview, entity.CoachingTypeManagerReview:
		return true
	}
	return false
}

func (u *usecase) Create(ctx context.Context, req CreateRequest) (*entity.CoachingSession, error) {
	if req.WorkspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if strings.TrimSpace(req.BDEmail) == "" {
		return nil, apperror.ValidationError("bd_email required")
	}
	if strings.TrimSpace(req.CoachEmail) == "" {
		return nil, apperror.ValidationError("coach_email required")
	}
	if !validSessionType(req.SessionType) {
		return nil, apperror.ValidationError("session_type must be peer_review|self_review|manager_review")
	}
	return u.repo.Insert(ctx, &entity.CoachingSession{
		WorkspaceID:        req.WorkspaceID,
		BDEmail:            strings.ToLower(req.BDEmail),
		CoachEmail:         strings.ToLower(req.CoachEmail),
		MasterDataID:       req.MasterDataID,
		ClaudeExtractionID: req.ClaudeExtractionID,
		SessionType:        req.SessionType,
		Status:             entity.CoachingStatusDraft,
	})
}

func (u *usecase) Get(ctx context.Context, workspaceID, id string) (*entity.CoachingSession, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	out, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, apperror.NotFound("coaching_session", id)
	}
	return out, nil
}

func (u *usecase) List(ctx context.Context, filter entity.CoachingSessionFilter) ([]entity.CoachingSession, int64, error) {
	if filter.WorkspaceID == "" {
		return nil, 0, apperror.ValidationError("workspace_id required")
	}
	return u.repo.List(ctx, filter)
}

func (u *usecase) Update(ctx context.Context, req UpdateRequest) (*entity.CoachingSession, error) {
	c, err := u.repo.GetByID(ctx, req.WorkspaceID, req.ID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, apperror.NotFound("coaching_session", req.ID)
	}
	if c.Status == entity.CoachingStatusReviewed {
		return nil, apperror.BadRequest("session already reviewed")
	}
	if req.BANTSClarityScore != nil {
		if err := checkScore(*req.BANTSClarityScore); err != nil {
			return nil, err
		}
		c.BANTSClarityScore = req.BANTSClarityScore
	}
	if req.DiscoveryDepthScore != nil {
		if err := checkScore(*req.DiscoveryDepthScore); err != nil {
			return nil, err
		}
		c.DiscoveryDepthScore = req.DiscoveryDepthScore
	}
	if req.ToneFitScore != nil {
		if err := checkScore(*req.ToneFitScore); err != nil {
			return nil, err
		}
		c.ToneFitScore = req.ToneFitScore
	}
	if req.NextStepClarityScore != nil {
		if err := checkScore(*req.NextStepClarityScore); err != nil {
			return nil, err
		}
		c.NextStepClarityScore = req.NextStepClarityScore
	}
	if req.Strengths != "" {
		c.Strengths = req.Strengths
	}
	if req.Improvements != "" {
		c.Improvements = req.Improvements
	}
	if req.ActionItems != "" {
		c.ActionItems = req.ActionItems
	}
	// Recompute overall as avg of non-nil sub-scores.
	c.OverallScore = averageScores(c)
	return u.repo.Update(ctx, c)
}

func (u *usecase) Submit(ctx context.Context, workspaceID, id, actor string) (*entity.CoachingSession, error) {
	c, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, apperror.NotFound("coaching_session", id)
	}
	if c.Status == entity.CoachingStatusReviewed {
		return nil, apperror.BadRequest("session already reviewed")
	}
	if c.OverallScore == nil {
		return nil, apperror.BadRequest("at least one score must be set before submitting")
	}
	c.Status = entity.CoachingStatusSubmitted
	return u.repo.Update(ctx, c)
}

func (u *usecase) Delete(ctx context.Context, workspaceID, id string) error {
	if workspaceID == "" || id == "" {
		return apperror.ValidationError("workspace_id and id required")
	}
	return u.repo.Delete(ctx, workspaceID, id)
}

func checkScore(v int) error {
	if v < 1 || v > 5 {
		return apperror.ValidationError("score must be in range 1–5")
	}
	return nil
}

func averageScores(c *entity.CoachingSession) *float64 {
	vals := []*int{c.BANTSClarityScore, c.DiscoveryDepthScore, c.ToneFitScore, c.NextStepClarityScore}
	sum, n := 0, 0
	for _, v := range vals {
		if v != nil {
			sum += *v
			n++
		}
	}
	if n == 0 {
		return nil
	}
	avg := float64(sum) / float64(n)
	return &avg
}
