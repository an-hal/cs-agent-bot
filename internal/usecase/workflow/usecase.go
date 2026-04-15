// Package workflow implements CRUD and canvas management for the workflow engine.
package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// Usecase is the workflow management interface.
type Usecase interface {
	List(ctx context.Context, workspaceID string, status *string) ([]entity.WorkflowListItem, error)
	GetByID(ctx context.Context, workspaceID, id string) (*entity.WorkflowFull, error)
	GetBySlug(ctx context.Context, workspaceID, slug string) (*entity.WorkflowFull, error)
	Create(ctx context.Context, actor string, w *entity.Workflow) (*entity.Workflow, error)
	Update(ctx context.Context, workspaceID, id, actor string, fields map[string]interface{}) error
	Delete(ctx context.Context, workspaceID, id string) error

	// Canvas
	SaveCanvas(ctx context.Context, workspaceID, id string, nodes []entity.WorkflowNode, edges []entity.WorkflowEdge) (*CanvasSaveResult, error)

	// Pipeline config
	SaveSteps(ctx context.Context, workspaceID, id string, steps []entity.WorkflowStep) error
	SaveTabs(ctx context.Context, workspaceID, id string, tabs []entity.PipelineTab) error
	SaveStats(ctx context.Context, workspaceID, id string, stats []entity.PipelineStat) error
	SaveColumns(ctx context.Context, workspaceID, id string, cols []entity.PipelineColumn) error
	GetConfig(ctx context.Context, workspaceID, id string) (*entity.WorkflowFull, error)

	// Step
	GetStepByKey(ctx context.Context, workspaceID, id, stepKey string) (*entity.WorkflowStep, error)
	UpdateStep(ctx context.Context, workspaceID, id, stepKey string, fields map[string]interface{}) error

	// For cron
	GetActiveForStage(ctx context.Context, workspaceID, stage string) (*entity.Workflow, error)
}

// CanvasSaveResult is the response payload for PUT /workflows/{id}/canvas.
type CanvasSaveResult struct {
	WorkflowID string    `json:"workflow_id"`
	NodeCount  int       `json:"node_count"`
	EdgeCount  int       `json:"edge_count"`
	SavedAt    time.Time `json:"saved_at"`
}

type usecase struct {
	repo   repository.WorkflowRepository
	logger zerolog.Logger
}

// New constructs a workflow Usecase.
func New(repo repository.WorkflowRepository, logger zerolog.Logger) Usecase {
	return &usecase{repo: repo, logger: logger}
}

// ─── List ─────────────────────────────────────────────────────────────────────

func (u *usecase) List(ctx context.Context, workspaceID string, status *string) ([]entity.WorkflowListItem, error) {
	items, err := u.repo.List(ctx, workspaceID, status)
	if err != nil {
		return nil, fmt.Errorf("workflow.List: %w", err)
	}
	if items == nil {
		items = []entity.WorkflowListItem{}
	}
	return items, nil
}

// ─── GetByID ──────────────────────────────────────────────────────────────────

func (u *usecase) GetByID(ctx context.Context, workspaceID, id string) (*entity.WorkflowFull, error) {
	full, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("workflow.GetByID: %w", err)
	}
	if full == nil {
		return nil, apperror.NotFound("workflow", "Workflow not found")
	}
	if full.WorkspaceID != workspaceID {
		return nil, apperror.NotFound("workflow", "Workflow not found")
	}
	return full, nil
}

// ─── GetBySlug ────────────────────────────────────────────────────────────────

func (u *usecase) GetBySlug(ctx context.Context, workspaceID, slug string) (*entity.WorkflowFull, error) {
	full, err := u.repo.GetBySlug(ctx, workspaceID, slug)
	if err != nil {
		return nil, fmt.Errorf("workflow.GetBySlug: %w", err)
	}
	if full == nil {
		return nil, apperror.NotFound("workflow", "Workflow not found")
	}
	return full, nil
}

// ─── Create ───────────────────────────────────────────────────────────────────

func (u *usecase) Create(ctx context.Context, actor string, w *entity.Workflow) (*entity.Workflow, error) {
	if w.Name == "" {
		return nil, apperror.ValidationError("name is required")
	}
	if w.Slug == "" {
		w.Slug = slugify(w.Name)
	}
	if w.Status == "" {
		w.Status = entity.WorkflowStatusDraft
	}
	if len(w.StageFilter) == 0 {
		w.StageFilter = []string{}
	}
	w.CreatedBy = &actor
	w.UpdatedBy = &actor

	if err := u.repo.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("workflow.Create: %w", err)
	}
	return w, nil
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (u *usecase) Update(ctx context.Context, workspaceID, id, actor string, fields map[string]interface{}) error {
	// Verify ownership
	full, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("workflow.Update: %w", err)
	}
	if full == nil || full.WorkspaceID != workspaceID {
		return apperror.NotFound("workflow", "Workflow not found")
	}
	fields["updated_by"] = actor
	if err := u.repo.Update(ctx, id, fields); err != nil {
		return fmt.Errorf("workflow.Update: %w", err)
	}
	return nil
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (u *usecase) Delete(ctx context.Context, workspaceID, id string) error {
	full, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("workflow.Delete: %w", err)
	}
	if full == nil || full.WorkspaceID != workspaceID {
		return apperror.NotFound("workflow", "Workflow not found")
	}
	if err := u.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("workflow.Delete: %w", err)
	}
	return nil
}

// ─── SaveCanvas ───────────────────────────────────────────────────────────────

func (u *usecase) SaveCanvas(ctx context.Context, workspaceID, id string, nodes []entity.WorkflowNode, edges []entity.WorkflowEdge) (*CanvasSaveResult, error) {
	full, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("workflow.SaveCanvas: %w", err)
	}
	if full == nil || full.WorkspaceID != workspaceID {
		return nil, apperror.NotFound("workflow", "Workflow not found")
	}
	if err := u.repo.SaveCanvas(ctx, id, nodes, edges); err != nil {
		return nil, fmt.Errorf("workflow.SaveCanvas: %w", err)
	}
	return &CanvasSaveResult{
		WorkflowID: id,
		NodeCount:  len(nodes),
		EdgeCount:  len(edges),
		SavedAt:    time.Now(),
	}, nil
}

// ─── Pipeline config ──────────────────────────────────────────────────────────

func (u *usecase) SaveSteps(ctx context.Context, workspaceID, id string, steps []entity.WorkflowStep) error {
	if err := u.assertOwner(ctx, workspaceID, id); err != nil {
		return err
	}
	return u.repo.SaveSteps(ctx, id, steps)
}

func (u *usecase) SaveTabs(ctx context.Context, workspaceID, id string, tabs []entity.PipelineTab) error {
	if err := u.assertOwner(ctx, workspaceID, id); err != nil {
		return err
	}
	return u.repo.SaveTabs(ctx, id, tabs)
}

func (u *usecase) SaveStats(ctx context.Context, workspaceID, id string, stats []entity.PipelineStat) error {
	if err := u.assertOwner(ctx, workspaceID, id); err != nil {
		return err
	}
	return u.repo.SaveStats(ctx, id, stats)
}

func (u *usecase) SaveColumns(ctx context.Context, workspaceID, id string, cols []entity.PipelineColumn) error {
	if err := u.assertOwner(ctx, workspaceID, id); err != nil {
		return err
	}
	return u.repo.SaveColumns(ctx, id, cols)
}

func (u *usecase) GetConfig(ctx context.Context, workspaceID, id string) (*entity.WorkflowFull, error) {
	if err := u.assertOwner(ctx, workspaceID, id); err != nil {
		return nil, err
	}
	cfg, err := u.repo.GetConfig(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("workflow.GetConfig: %w", err)
	}
	return cfg, nil
}

// ─── Steps ────────────────────────────────────────────────────────────────────

func (u *usecase) GetStepByKey(ctx context.Context, workspaceID, id, stepKey string) (*entity.WorkflowStep, error) {
	if err := u.assertOwner(ctx, workspaceID, id); err != nil {
		return nil, err
	}
	step, err := u.repo.GetStepByKey(ctx, id, stepKey)
	if err != nil {
		return nil, fmt.Errorf("workflow.GetStepByKey: %w", err)
	}
	if step == nil {
		return nil, apperror.NotFound("step", "Step not found")
	}
	return step, nil
}

func (u *usecase) UpdateStep(ctx context.Context, workspaceID, id, stepKey string, fields map[string]interface{}) error {
	if err := u.assertOwner(ctx, workspaceID, id); err != nil {
		return err
	}
	return u.repo.UpdateStep(ctx, id, stepKey, fields)
}

// ─── GetActiveForStage ────────────────────────────────────────────────────────

// GetActiveForStage returns the first active workflow whose stage_filter includes
// the given stage. Used by the workflow cron runner.
func (u *usecase) GetActiveForStage(ctx context.Context, workspaceID, stage string) (*entity.Workflow, error) {
	items, err := u.repo.List(ctx, workspaceID, strPtr(string(entity.WorkflowStatusActive)))
	if err != nil {
		return nil, fmt.Errorf("workflow.GetActiveForStage: %w", err)
	}
	for _, item := range items {
		for _, s := range item.StageFilter {
			if s == stage {
				return &item.Workflow, nil
			}
		}
	}
	return nil, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func (u *usecase) assertOwner(ctx context.Context, workspaceID, id string) error {
	full, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("workflow.assertOwner: %w", err)
	}
	if full == nil || full.WorkspaceID != workspaceID {
		return apperror.NotFound("workflow", "Workflow not found")
	}
	return nil
}

// slugify creates a URL-safe slug from a workflow name.
func slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func strPtr(s string) *string { return &s }
