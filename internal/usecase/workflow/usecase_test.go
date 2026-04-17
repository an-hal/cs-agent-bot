package workflow_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/workflow"
	"github.com/rs/zerolog"
)

// ─── Stub repository ──────────────────────────────────────────────────────────

type stubRepo struct {
	workflows  map[string]*entity.WorkflowFull
	listItems  []entity.WorkflowListItem
	saveCanvasErr error
}

func (s *stubRepo) List(_ context.Context, _ string, _ *string) ([]entity.WorkflowListItem, error) {
	return s.listItems, nil
}

func (s *stubRepo) GetByID(_ context.Context, id string) (*entity.WorkflowFull, error) {
	if wf, ok := s.workflows[id]; ok {
		return wf, nil
	}
	return nil, nil
}

func (s *stubRepo) GetBySlug(_ context.Context, _, slug string) (*entity.WorkflowFull, error) {
	for _, wf := range s.workflows {
		if wf.Slug == slug {
			return wf, nil
		}
	}
	return nil, nil
}

func (s *stubRepo) Create(_ context.Context, w *entity.Workflow) error {
	w.ID = "new-id"
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()
	return nil
}

func (s *stubRepo) Update(_ context.Context, _ string, _ map[string]interface{}) error { return nil }
func (s *stubRepo) Delete(_ context.Context, _ string) error                           { return nil }

func (s *stubRepo) SaveCanvas(_ context.Context, _ string, _ []entity.WorkflowNode, _ []entity.WorkflowEdge) error {
	return s.saveCanvasErr
}

func (s *stubRepo) SaveSteps(_ context.Context, _ string, _ []entity.WorkflowStep) error    { return nil }
func (s *stubRepo) SaveTabs(_ context.Context, _ string, _ []entity.PipelineTab) error      { return nil }
func (s *stubRepo) SaveStats(_ context.Context, _ string, _ []entity.PipelineStat) error    { return nil }
func (s *stubRepo) SaveColumns(_ context.Context, _ string, _ []entity.PipelineColumn) error{ return nil }

func (s *stubRepo) GetConfig(_ context.Context, workflowID string) (*entity.WorkflowFull, error) {
	if wf, ok := s.workflows[workflowID]; ok {
		return wf, nil
	}
	return nil, nil
}

func (s *stubRepo) GetStepByKey(_ context.Context, workflowID, _ string) (*entity.WorkflowStep, error) {
	return nil, nil
}
func (s *stubRepo) UpdateStep(_ context.Context, _, _ string, _ map[string]interface{}) error {
	return nil
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestCreate_RequiresName(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())

	_, err := uc.Create(context.Background(), "actor@test.com", &entity.Workflow{
		WorkspaceID: "ws-1",
	})
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestCreate_SetsSlugAndCreatedBy(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())

	w := &entity.Workflow{
		WorkspaceID: "ws-1",
		Name:        "My Test Workflow",
	}
	created, err := uc.Create(context.Background(), "actor@test.com", w)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Slug == "" {
		t.Error("expected slug to be auto-generated")
	}
	if created.CreatedBy == nil || *created.CreatedBy != "actor@test.com" {
		t.Errorf("expected created_by actor@test.com, got %v", created.CreatedBy)
	}
}

func TestGetByID_NotFoundReturnsError(t *testing.T) {
	t.Parallel()

	uc := workflow.New(&stubRepo{workflows: map[string]*entity.WorkflowFull{}}, zerolog.Nop())

	_, err := uc.GetByID(context.Background(), "ws-1", "nonexistent-id")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestGetByID_WrongWorkspaceReturnsError(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-1": {
				Workflow: entity.Workflow{
					ID:          "wf-1",
					WorkspaceID: "ws-correct",
					Name:        "Test",
					Status:      entity.WorkflowStatusActive,
				},
			},
		},
	}

	uc := workflow.New(repo, zerolog.Nop())
	_, err := uc.GetByID(context.Background(), "ws-wrong", "wf-1")
	if err == nil {
		t.Fatal("expected access denied, got nil")
	}
}

func TestSaveCanvas_ReturnsCorrectCounts(t *testing.T) {
	t.Parallel()

	repo := &stubRepo{
		workflows: map[string]*entity.WorkflowFull{
			"wf-1": {
				Workflow: entity.Workflow{
					ID:          "wf-1",
					WorkspaceID: "ws-1",
					Name:        "Test WF",
					Status:      entity.WorkflowStatusActive,
				},
			},
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	nodes := make([]entity.WorkflowNode, 5)
	edges := make([]entity.WorkflowEdge, 3)

	result, err := uc.SaveCanvas(context.Background(), "ws-1", "wf-1", nodes, edges)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NodeCount != 5 {
		t.Errorf("expected 5 nodes, got %d", result.NodeCount)
	}
	if result.EdgeCount != 3 {
		t.Errorf("expected 3 edges, got %d", result.EdgeCount)
	}
}

func TestGetActiveForStage_FindsMatchingWorkflow(t *testing.T) {
	t.Parallel()

	statusActive := string(entity.WorkflowStatusActive)
	repo := &stubRepo{
		listItems: []entity.WorkflowListItem{
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
	_ = statusActive
	uc := workflow.New(repo, zerolog.Nop())

	wf, err := uc.GetActiveForStage(context.Background(), "ws-1", "CLIENT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf == nil {
		t.Fatal("expected to find a workflow for CLIENT stage")
	}
	if wf.ID != "wf-ae" {
		t.Errorf("expected wf-ae, got %s", wf.ID)
	}
}

func TestGetActiveForStage_ReturnsNilWhenNotFound(t *testing.T) {
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
		},
	}
	uc := workflow.New(repo, zerolog.Nop())

	wf, err := uc.GetActiveForStage(context.Background(), "ws-1", "PROSPECT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf != nil {
		t.Errorf("expected nil, got %v", wf.ID)
	}
}
