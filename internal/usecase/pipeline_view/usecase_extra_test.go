package pipeline_view_test

// usecase_extra_test.go — additional pipeline_view tests to raise coverage.
// Covers: stat computation error fallback, empty tab key defaults to "all",
// multiple stats computed, list error propagation.

import (
	"context"
	"errors"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	pipelineview "github.com/Sejutacita/cs-agent-bot/internal/usecase/pipeline_view"
	"github.com/rs/zerolog"
)

// ─── Stat error falls back to "0" ────────────────────────────────────────────

func TestGetData_StatComputationError_FallsBackToZero(t *testing.T) {
	t.Parallel()

	repo := &errorStatRepo{
		records: []entity.MasterData{{ID: "1", CompanyName: "PT Alpha"}},
		total:   1,
	}
	uc := pipelineview.New(repo, zerolog.Nop())

	wf := &entity.WorkflowFull{
		Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"},
		Stats: []entity.PipelineStat{
			{StatKey: "revenue", Metric: "sum:final_price"},
		},
	}

	resp, err := uc.GetData(context.Background(), "ws-1", wf, "", "", pagination.Params{Limit: 10}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Even though stat computation failed, the response must succeed.
	if resp.Stats["revenue"].Value != "0" {
		t.Errorf("expected stat fallback '0', got %q", resp.Stats["revenue"].Value)
	}
}

// ─── Multiple stats computed ──────────────────────────────────────────────────

func TestGetData_MultipleStats_AllComputed(t *testing.T) {
	t.Parallel()

	repo := &stubPipelineRepo{
		records: []entity.MasterData{},
		total:   0,
		statVals: map[string]string{
			"count":           "10",
			"sum:final_price": "5000000",
			"avg:contract_months": "12",
		},
	}
	uc := pipelineview.New(repo, zerolog.Nop())

	wf := &entity.WorkflowFull{
		Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"},
		Stats: []entity.PipelineStat{
			{StatKey: "total_clients", Metric: "count"},
			{StatKey: "total_revenue", Metric: "sum:final_price"},
			{StatKey: "avg_contract", Metric: "avg:contract_months"},
		},
	}

	resp, err := uc.GetData(context.Background(), "ws-1", wf, "", "", pagination.Params{Limit: 10}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Stats["total_clients"].Value != "10" {
		t.Errorf("expected 'total_clients'=10, got %s", resp.Stats["total_clients"].Value)
	}
	if resp.Stats["total_revenue"].Value != "5000000" {
		t.Errorf("expected 'total_revenue'=5000000, got %s", resp.Stats["total_revenue"].Value)
	}
	if resp.Stats["avg_contract"].Value != "12" {
		t.Errorf("expected 'avg_contract'=12, got %s", resp.Stats["avg_contract"].Value)
	}
}

// ─── List data error propagates ───────────────────────────────────────────────

func TestGetData_ListDataError_ReturnsError(t *testing.T) {
	t.Parallel()

	repo := &stubPipelineRepo{
		listErr: errors.New("database connection lost"),
	}
	uc := pipelineview.New(repo, zerolog.Nop())

	_, err := uc.GetData(context.Background(), "ws-1", &entity.WorkflowFull{
		Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"},
	}, "", "", pagination.Params{}, "", "")

	if err == nil {
		t.Fatal("expected error for list data failure, got nil")
	}
}

// ─── Empty tab key defaults to "all" ─────────────────────────────────────────

func TestGetData_EmptyTabKey_DefaultsToAllFilter(t *testing.T) {
	t.Parallel()

	var capturedReq repository.PipelineDataRequest
	repo := &capturingRepo{
		onListData: func(req repository.PipelineDataRequest) {
			capturedReq = req
		},
	}

	uc := pipelineview.New(repo, zerolog.Nop())
	wf := &entity.WorkflowFull{
		Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"},
		Tabs: []entity.PipelineTab{
			{TabKey: "active", Filter: "bot_active"},
		},
	}

	_, err := uc.GetData(context.Background(), "ws-1", wf, "", "", pagination.Params{}, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.TabFilter != "all" {
		t.Errorf("empty tab key: expected 'all', got %q", capturedReq.TabFilter)
	}
}

// ─── StageFilter passed to repo ───────────────────────────────────────────────

func TestGetData_StageFilterPassedToRepo(t *testing.T) {
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
			StageFilter: []string{"CLIENT", "DORMANT"},
		},
	}

	_, err := uc.GetData(context.Background(), "ws-1", wf, "", "", pagination.Params{Limit: 20}, "updated_at", "desc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(capturedReq.StageFilter) != 2 {
		t.Errorf("expected 2 stage filters, got %d", len(capturedReq.StageFilter))
	}
}

// ─── Sort and search params passed to repo ───────────────────────────────────

func TestGetData_SortAndSearchPassedToRepo(t *testing.T) {
	t.Parallel()

	var capturedReq repository.PipelineDataRequest
	repo := &capturingRepo{
		onListData: func(req repository.PipelineDataRequest) {
			capturedReq = req
		},
	}

	uc := pipelineview.New(repo, zerolog.Nop())
	wf := &entity.WorkflowFull{
		Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"},
	}

	_, err := uc.GetData(context.Background(), "ws-1", wf, "", "PT Alpha", pagination.Params{Limit: 10}, "company_name", "asc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.Search != "PT Alpha" {
		t.Errorf("expected search='PT Alpha', got %q", capturedReq.Search)
	}
	if capturedReq.SortBy != "company_name" {
		t.Errorf("expected sort_by='company_name', got %q", capturedReq.SortBy)
	}
	if capturedReq.SortDir != "asc" {
		t.Errorf("expected sort_dir='asc', got %q", capturedReq.SortDir)
	}
}

// ─── No stats configured — empty stats map ───────────────────────────────────

func TestGetData_NoStats_ReturnsEmptyStatsMap(t *testing.T) {
	t.Parallel()

	repo := &stubPipelineRepo{records: []entity.MasterData{}, total: 0}
	uc := pipelineview.New(repo, zerolog.Nop())

	resp, err := uc.GetData(context.Background(), "ws-1", &entity.WorkflowFull{
		Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"},
		Stats:    nil, // no stats configured
	}, "", "", pagination.Params{}, "", "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Stats == nil {
		t.Error("expected non-nil stats map")
	}
	if len(resp.Stats) != 0 {
		t.Errorf("expected 0 stats, got %d", len(resp.Stats))
	}
}

// ─── Total returned from repo ─────────────────────────────────────────────────

func TestGetData_TotalMatchesRepoCount(t *testing.T) {
	t.Parallel()

	repo := &stubPipelineRepo{
		records: []entity.MasterData{
			{ID: "1"}, {ID: "2"}, {ID: "3"},
		},
		total: 42, // total is larger than page (simulating pagination)
	}
	uc := pipelineview.New(repo, zerolog.Nop())

	resp, err := uc.GetData(context.Background(), "ws-1", &entity.WorkflowFull{
		Workflow: entity.Workflow{ID: "wf-1", WorkspaceID: "ws-1"},
	}, "", "", pagination.Params{Limit: 3, Offset: 0}, "", "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 42 {
		t.Errorf("expected total=42, got %d", resp.Total)
	}
	if len(resp.Data) != 3 {
		t.Errorf("expected 3 records in page, got %d", len(resp.Data))
	}
}

// ─── Stub helpers ─────────────────────────────────────────────────────────────

// errorStatRepo returns an error from ComputeStat but succeeds on ListData.
type errorStatRepo struct {
	records []entity.MasterData
	total   int64
}

func (r *errorStatRepo) ListData(_ context.Context, _ repository.PipelineDataRequest) ([]entity.MasterData, int64, error) {
	return r.records, r.total, nil
}

func (r *errorStatRepo) ComputeStat(_ context.Context, _, _ string) (string, error) {
	return "", errors.New("stat computation failed")
}
