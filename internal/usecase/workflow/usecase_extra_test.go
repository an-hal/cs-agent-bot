package workflow_test

// usecase_extra_test.go — additional tests to raise workflow usecase coverage.
// Covers: Update, Delete, SaveCanvas errors, SaveSteps/Tabs/Stats/Columns,
// GetConfig, GetStepByKey, UpdateStep, GetBySlug, List, and slugify edge cases.

import (
	"context"
	"errors"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/workflow"
	"github.com/rs/zerolog"
)

// ─── Update ───────────────────────────────────────────────────────────────────

func TestUpdate_NotFoundReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())

	err := uc.Update(context.Background(), "ws-1", "nonexistent-id", "actor@test.com", map[string]interface{}{
		"name": "new name",
	})
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
	if !apperror.IsNotFound(err) {
		t.Errorf("expected IsNotFound, got: %v", err)
	}
}

func TestUpdate_WrongWorkspaceReturnsNotFound(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-1": {Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-correct", Name: "WF"}},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	err := uc.Update(context.Background(), "ws-wrong", "wf-1", "actor", map[string]interface{}{
		"name": "updated",
	})
	if err == nil {
		t.Fatal("expected not found error for wrong workspace, got nil")
	}
}

func TestUpdate_SetsUpdatedByField(t *testing.T) {
	t.Parallel()

	captured := map[string]interface{}{}
	repo := &capturingUpdateRepo{
		stubRepo: &stubRepo{
			workflows: map[string]*entity.WorkflowFull{
				"wf-1": {Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1", Name: "WF"}},
			},
		},
		onUpdate: func(id string, fields map[string]interface{}) {
			for k, v := range fields {
				captured[k] = v
			}
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	err := uc.Update(context.Background(), "ws-1", "wf-1", "editor@test.com", map[string]interface{}{
		"name": "Updated",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured["updated_by"] != "editor@test.com" {
		t.Errorf("expected updated_by='editor@test.com', got %v", captured["updated_by"])
	}
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestDelete_NotFoundReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())

	err := uc.Delete(context.Background(), "ws-1", "nonexistent")
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestDelete_WrongWorkspaceReturnsError(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-2": {Workflow: entity.Workflow{ID: "wf-2", WorkspaceID: "ws-correct"}},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	err := uc.Delete(context.Background(), "ws-wrong", "wf-2")
	if err == nil {
		t.Fatal("expected not found error for wrong workspace, got nil")
	}
}

func TestDelete_CorrectWorkspace_Succeeds(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-del": {Workflow: entity.Workflow{ID: "wf-del", WorkspaceID: "ws-1", Name: "Delete Me"}},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	err := uc.Delete(context.Background(), "ws-1", "wf-del")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── SaveCanvas — error path ──────────────────────────────────────────────────

func TestSaveCanvas_RepoError_PropagatesError(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-err": {Workflow: entity.Workflow{ID: "wf-err", WorkspaceID: "ws-1"}},
		},
		saveCanvasErr: errors.New("db write failed"),
	}
	uc := workflow.New(repo, zerolog.Nop())

	_, err := uc.SaveCanvas(context.Background(), "ws-1", "wf-err",
		[]entity.WorkflowNode{}, []entity.WorkflowEdge{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSaveCanvas_NotFoundWorkflow_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())

	_, err := uc.SaveCanvas(context.Background(), "ws-1", "nonexistent",
		[]entity.WorkflowNode{}, []entity.WorkflowEdge{})
	if err == nil {
		t.Fatal("expected not found error, got nil")
	}
}

func TestSaveCanvas_EmptyNodesAndEdges_ReturnsZeroCounts(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-1": {Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1", Name: "WF"}},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	result, err := uc.SaveCanvas(context.Background(), "ws-1", "wf-1",
		[]entity.WorkflowNode{}, []entity.WorkflowEdge{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NodeCount != 0 {
		t.Errorf("expected 0 nodes, got %d", result.NodeCount)
	}
	if result.EdgeCount != 0 {
		t.Errorf("expected 0 edges, got %d", result.EdgeCount)
	}
}

// ─── Pipeline config methods ──────────────────────────────────────────────────

func TestSaveSteps_NotFoundWorkflow_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())
	err := uc.SaveSteps(context.Background(), "ws-1", "nonexistent", []entity.WorkflowStep{})
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
}

func TestSaveSteps_ValidWorkflow_Succeeds(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-1": {Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"}},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	err := uc.SaveSteps(context.Background(), "ws-1", "wf-1", []entity.WorkflowStep{
		{StepKey: "step-1", Label: "Onboarding", Phase: "P0"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveTabs_NotFoundWorkflow_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())
	err := uc.SaveTabs(context.Background(), "ws-1", "nonexistent", []entity.PipelineTab{})
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
}

func TestSaveStats_NotFoundWorkflow_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())
	err := uc.SaveStats(context.Background(), "ws-1", "nonexistent", []entity.PipelineStat{})
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
}

func TestSaveColumns_NotFoundWorkflow_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())
	err := uc.SaveColumns(context.Background(), "ws-1", "nonexistent", []entity.PipelineColumn{})
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
}

func TestSaveColumns_ValidWorkflow_Succeeds(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-col": {Workflow: entity.Workflow{ID: "wf-col", WorkspaceID: "ws-1"}},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	err := uc.SaveColumns(context.Background(), "ws-1", "wf-col", []entity.PipelineColumn{
		{ColumnKey: "company_name", Field: "company_name", Label: "Company"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── GetConfig ────────────────────────────────────────────────────────────────

func TestGetConfig_ValidWorkflow_ReturnsConfig(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-cfg": {
				Workflow: entity.Workflow{ID: "wf-cfg", WorkspaceID: "ws-1", Name: "Config WF"},
				Steps:    []entity.WorkflowStep{{StepKey: "s1", Label: "Step 1", Phase: "P0"}},
			},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	cfg, err := uc.GetConfig(context.Background(), "ws-1", "wf-cfg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
}

func TestGetConfig_NotFoundWorkflow_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())

	_, err := uc.GetConfig(context.Background(), "ws-1", "nonexistent")
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
}

// ─── GetStepByKey ─────────────────────────────────────────────────────────────

func TestGetStepByKey_NotFoundWorkflow_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())

	_, err := uc.GetStepByKey(context.Background(), "ws-1", "nonexistent", "step-1")
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
}

func TestGetStepByKey_ValidWorkflow_StepNotFound_ReturnsError(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-1": {Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"}},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	// stubRepo.GetStepByKey always returns nil, nil, so the usecase returns NotFound.
	_, err := uc.GetStepByKey(context.Background(), "ws-1", "wf-1", "missing-step")
	if err == nil {
		t.Fatal("expected not found for missing step, got nil")
	}
}

// ─── UpdateStep ───────────────────────────────────────────────────────────────

func TestUpdateStep_NotFoundWorkflow_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())

	err := uc.UpdateStep(context.Background(), "ws-1", "nonexistent", "step-1", map[string]interface{}{
		"timing": "H-30",
	})
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
}

func TestUpdateStep_ValidWorkflow_Succeeds(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-1": {Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"}},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	err := uc.UpdateStep(context.Background(), "ws-1", "wf-1", "step-1", map[string]interface{}{
		"timing": "H-30",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── GetBySlug ────────────────────────────────────────────────────────────────

func TestGetBySlug_Found_ReturnsWorkflow(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-slug": {Workflow: entity.Workflow{
				ID:          "wf-slug",
				WorkspaceID: "ws-1",
				Slug:        "ae-lifecycle",
				Name:        "AE Lifecycle",
			}},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	wf, err := uc.GetBySlug(context.Background(), "ws-1", "ae-lifecycle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf == nil {
		t.Fatal("expected workflow, got nil")
	}
	if wf.Slug != "ae-lifecycle" {
		t.Errorf("expected slug 'ae-lifecycle', got %s", wf.Slug)
	}
}

func TestGetBySlug_NotFound_ReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())

	_, err := uc.GetBySlug(context.Background(), "ws-1", "nonexistent-slug")
	if err == nil {
		t.Fatal("expected not found, got nil")
	}
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestList_ReturnsAllItems(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		listItems: []entity.WorkflowListItem{
			{Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1", Status: entity.WorkflowStatusActive}},
			{Workflow: entity.Workflow{ID: "wf-2", WorkspaceID: "ws-1", Status: entity.WorkflowStatusDraft}},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	items, err := uc.List(context.Background(), "ws-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestList_NilResult_ReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{listItems: nil}
	uc := workflow.New(repo, zerolog.Nop())

	items, err := uc.List(context.Background(), "ws-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if items == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

// ─── Create — auto-slug from name ─────────────────────────────────────────────

func TestCreate_SlugifiesName_SpecialCharsStripped(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{workflows: map[string]*entity.WorkflowFull{}}
	uc := workflow.New(repo, zerolog.Nop())

	w := &entity.Workflow{
		WorkspaceID: "ws-1",
		Name:        "AE Lifecycle 2026!",
	}
	created, err := uc.Create(context.Background(), "actor@test.com", w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Slug should be lowercase, spaces replaced with hyphens, special chars removed.
	if created.Slug == "" {
		t.Fatal("expected non-empty slug")
	}
	for _, r := range created.Slug {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			t.Errorf("slug contains invalid character %q: %s", r, created.Slug)
			break
		}
	}
}

func TestCreate_ExplicitSlugNotOverridden(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{workflows: map[string]*entity.WorkflowFull{}}
	uc := workflow.New(repo, zerolog.Nop())

	w := &entity.Workflow{
		WorkspaceID: "ws-1",
		Name:        "Some Workflow",
		Slug:        "custom-slug",
	}
	created, err := uc.Create(context.Background(), "actor@test.com", w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Slug != "custom-slug" {
		t.Errorf("expected slug 'custom-slug', got %q", created.Slug)
	}
}

func TestCreate_DefaultStatus_IsDraft(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{workflows: map[string]*entity.WorkflowFull{}}
	uc := workflow.New(repo, zerolog.Nop())

	w := &entity.Workflow{
		WorkspaceID: "ws-1",
		Name:        "Draft WF",
	}
	created, err := uc.Create(context.Background(), "actor@test.com", w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Status != entity.WorkflowStatusDraft {
		t.Errorf("expected status 'draft', got %q", created.Status)
	}
}

func TestCreate_EmptyStageFilter_InitialisedToEmptySlice(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{workflows: map[string]*entity.WorkflowFull{}}
	uc := workflow.New(repo, zerolog.Nop())

	w := &entity.Workflow{
		WorkspaceID: "ws-1",
		Name:        "No Stage WF",
		StageFilter: nil, // explicitly nil
	}
	created, err := uc.Create(context.Background(), "actor@test.com", w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.StageFilter == nil {
		t.Error("expected non-nil StageFilter (empty slice), got nil")
	}
}

// ─── GetActiveForStage — multiple workflows ───────────────────────────────────

func TestGetActiveForStage_MultipleWorkflows_ReturnsFirstMatch(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		listItems: []entity.WorkflowListItem{
			{
				Workflow: entity.Workflow{
					ID:          "wf-sdr",
					WorkspaceID: "ws-1",
					Status:      entity.WorkflowStatusActive,
					StageFilter: []string{"LEAD"},
				},
			},
			{
				Workflow: entity.Workflow{
					ID:          "wf-ae",
					WorkspaceID: "ws-1",
					Status:      entity.WorkflowStatusActive,
					StageFilter: []string{"CLIENT"},
				},
			},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	wf, err := uc.GetActiveForStage(context.Background(), "ws-1", "CLIENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf == nil {
		t.Fatal("expected workflow, got nil")
	}
	if wf.ID != "wf-ae" {
		t.Errorf("expected 'wf-ae', got %s", wf.ID)
	}
}

func TestGetActiveForStage_WorkflowWithMultipleStages_MatchesAny(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		listItems: []entity.WorkflowListItem{
			{
				Workflow: entity.Workflow{
					ID:          "wf-multi",
					WorkspaceID: "ws-1",
					Status:      entity.WorkflowStatusActive,
					StageFilter: []string{"LEAD", "PROSPECT", "CLIENT"},
				},
			},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	for _, stage := range []string{"LEAD", "PROSPECT", "CLIENT"} {
		wf, err := uc.GetActiveForStage(context.Background(), "ws-1", stage)
		if err != nil {
			t.Fatalf("stage %s: unexpected error: %v", stage, err)
		}
		if wf == nil {
			t.Errorf("stage %s: expected workflow, got nil", stage)
		}
	}
}

func TestGetActiveForStage_EmptyList_ReturnsNil(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{listItems: []entity.WorkflowListItem{}}
	uc := workflow.New(repo, zerolog.Nop())

	wf, err := uc.GetActiveForStage(context.Background(), "ws-1", "CLIENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf != nil {
		t.Errorf("expected nil, got workflow %s", wf.ID)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// capturingUpdateRepo wraps stubRepo and captures Update calls.
type capturingUpdateRepo struct {
	*stubRepo
	onUpdate func(id string, fields map[string]interface{})
}

func (r *capturingUpdateRepo) Update(_ context.Context, id string, fields map[string]interface{}) error {
	if r.onUpdate != nil {
		r.onUpdate(id, fields)
	}
	return nil
}
