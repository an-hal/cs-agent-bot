package pipeline_view_test

import (
	"context"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	pipelineview "github.com/Sejutacita/cs-agent-bot/internal/usecase/pipeline_view"
	"github.com/rs/zerolog"
)

// ─── Stub repository ──────────────────────────────────────────────────────────

type stubPipelineRepo struct {
	records  []entity.MasterData
	total    int64
	listErr  error
	statVals map[string]string
}

func (s *stubPipelineRepo) ListData(_ context.Context, req repository.PipelineDataRequest) ([]entity.MasterData, int64, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return s.records, s.total, nil
}

func (s *stubPipelineRepo) ComputeStat(_ context.Context, _, metric string) (string, error) {
	if v, ok := s.statVals[metric]; ok {
		return v, nil
	}
	return "0", nil
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestGetData_ReturnsRecordsAndStats(t *testing.T) {
	t.Parallel()

	repo := &stubPipelineRepo{
		records: []entity.MasterData{
			{ID: "1", CompanyName: "PT Alpha"},
			{ID: "2", CompanyName: "PT Beta"},
		},
		total: 2,
		statVals: map[string]string{
			"count": "2",
			"sum:final_price": "50000000",
		},
	}

	uc := pipelineview.New(repo, zerolog.Nop())
	wf := &entity.WorkflowFull{
		Workflow: entity.Workflow{
			ID:          "wf-1",
			WorkspaceID: "ws-1",
			StageFilter: []string{"CLIENT"},
		},
		Tabs: []entity.PipelineTab{
			{TabKey: "all", Label: "Semua", Filter: "all"},
			{TabKey: "aktif", Label: "Bot Aktif", Filter: "bot_active"},
		},
		Stats: []entity.PipelineStat{
			{StatKey: "total", Metric: "count"},
			{StatKey: "revenue", Metric: "sum:final_price"},
		},
	}

	resp, err := uc.GetData(context.Background(), "ws-1", wf, "all", "", pagination.Params{Limit: 20}, "updated_at", "desc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 records, got %d", len(resp.Data))
	}
	if resp.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Total)
	}
	if resp.Stats["total"].Value != "2" {
		t.Errorf("expected stat total=2, got %s", resp.Stats["total"].Value)
	}
	if resp.Stats["revenue"].Value != "50000000" {
		t.Errorf("expected stat revenue=50000000, got %s", resp.Stats["revenue"].Value)
	}
}

func TestGetData_NilWorkflowReturnsError(t *testing.T) {
	t.Parallel()

	uc := pipelineview.New(&stubPipelineRepo{}, zerolog.Nop())
	_, err := uc.GetData(context.Background(), "ws-1", nil, "", "", pagination.Params{}, "", "")
	if err == nil {
		t.Fatal("expected error for nil workflow, got nil")
	}
}

func TestGetData_TabFilterResolvedCorrectly(t *testing.T) {
	t.Parallel()

	var capturedReq repository.PipelineDataRequest
	repo := &capturingRepo{
		onListData: func(req repository.PipelineDataRequest) {
			capturedReq = req
		},
	}

	uc := pipelineview.New(repo, zerolog.Nop())
	wf := &entity.WorkflowFull{
		Workflow: entity.Workflow{
			ID:          "wf-1",
			WorkspaceID: "ws-1",
			StageFilter: []string{"LEAD"},
		},
		Tabs: []entity.PipelineTab{
			{TabKey: "risk", Label: "Perhatian", Filter: "risk"},
		},
	}

	_, err := uc.GetData(context.Background(), "ws-1", wf, "risk", "", pagination.Params{Limit: 10}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.TabFilter != "risk" {
		t.Errorf("expected tab filter 'risk', got %q", capturedReq.TabFilter)
	}
}

func TestGetData_UnknownTabDefaultsToAll(t *testing.T) {
	t.Parallel()

	var capturedReq repository.PipelineDataRequest
	repo := &capturingRepo{
		onListData: func(req repository.PipelineDataRequest) {
			capturedReq = req
		},
	}

	uc := pipelineview.New(repo, zerolog.Nop())
	wf := &entity.WorkflowFull{
		Workflow: entity.Workflow{
			ID:          "wf-1",
			WorkspaceID: "ws-1",
		},
		Tabs: []entity.PipelineTab{
			{TabKey: "aktif", Filter: "bot_active"},
		},
	}

	_, err := uc.GetData(context.Background(), "ws-1", wf, "nonexistent", "", pagination.Params{}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.TabFilter != "all" {
		t.Errorf("expected fallback tab filter 'all', got %q", capturedReq.TabFilter)
	}
}

func TestGetData_EmptyRecordsReturnEmptySlice(t *testing.T) {
	t.Parallel()

	repo := &stubPipelineRepo{records: nil, total: 0}
	uc := pipelineview.New(repo, zerolog.Nop())

	resp, err := uc.GetData(context.Background(), "ws-1", &entity.WorkflowFull{
		Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"},
	}, "", "", pagination.Params{}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected non-nil slice for empty result")
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected 0 records, got %d", len(resp.Data))
	}
}

// ─── capturingRepo captures the last ListData request for assertion ───────────

type capturingRepo struct {
	onListData func(repository.PipelineDataRequest)
}

func (c *capturingRepo) ListData(_ context.Context, req repository.PipelineDataRequest) ([]entity.MasterData, int64, error) {
	if c.onListData != nil {
		c.onListData(req)
	}
	return []entity.MasterData{}, 0, nil
}

func (c *capturingRepo) ComputeStat(_ context.Context, _, _ string) (string, error) {
	return "0", nil
}
