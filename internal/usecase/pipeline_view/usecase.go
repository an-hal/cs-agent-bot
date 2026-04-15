// Package pipeline_view implements the pipeline data query usecase for the
// workflow engine (feat/06). It combines workflow stage_filter with tab DSL
// filters to return paginated master_data records and computed stat values.
package pipeline_view

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/filterdsl"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// StatResult holds the computed value for a single stat card.
type StatResult struct {
	Value string `json:"value"`
}

// DataResponse is the structured response for GET /workflows/{id}/data.
type DataResponse struct {
	Data  []entity.MasterData       `json:"data"`
	Meta  pagination.Params         `json:"meta"`
	Total int64                     `json:"total"`
	Stats map[string]StatResult     `json:"stats"`
}

// Usecase is the pipeline view interface.
type Usecase interface {
	// GetData returns master_data filtered by workflow stage_filter + tab DSL.
	// workflowFull provides stage_filter, tabs, and stats config.
	GetData(ctx context.Context, workspaceID string, wf *entity.WorkflowFull, tabKey, search string, p pagination.Params, sortBy, sortDir string) (*DataResponse, error)
}

type usecase struct {
	pipelineRepo repository.PipelineViewRepository
	logger       zerolog.Logger
}

// New constructs a pipeline view Usecase.
func New(pipelineRepo repository.PipelineViewRepository, logger zerolog.Logger) Usecase {
	return &usecase{pipelineRepo: pipelineRepo, logger: logger}
}

// GetData queries master_data records for the given workflow, applying tab
// filters, search, sort, and pagination. Stats are computed per the workflow's
// stat card metric DSL.
func (u *usecase) GetData(ctx context.Context, workspaceID string, wf *entity.WorkflowFull, tabKey, search string, p pagination.Params, sortBy, sortDir string) (*DataResponse, error) {
	if wf == nil {
		return nil, apperror.NotFound("workflow", "Workflow not found")
	}

	// Resolve tab filter DSL from the workflow's configured tabs.
	tabFilter := resolveTabFilter(wf.Tabs, tabKey)

	req := repository.PipelineDataRequest{
		WorkspaceID: workspaceID,
		StageFilter: wf.StageFilter,
		TabFilter:   tabFilter,
		Search:      search,
		Pagination:  p,
		SortBy:      sortBy,
		SortDir:     sortDir,
	}

	records, total, err := u.pipelineRepo.ListData(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("pipelineView.GetData: %w", err)
	}
	if records == nil {
		records = []entity.MasterData{}
	}

	// Compute stats for each configured stat card.
	stats := make(map[string]StatResult, len(wf.Stats))
	for _, s := range wf.Stats {
		val, statErr := u.pipelineRepo.ComputeStat(ctx, workspaceID, s.Metric)
		if statErr != nil {
			u.logger.Warn().Err(statErr).Str("metric", s.Metric).Msg("stat computation failed, using 0")
			val = "0"
		}
		stats[s.StatKey] = StatResult{Value: val}
	}

	return &DataResponse{
		Data:  records,
		Meta:  p,
		Total: total,
		Stats: stats,
	}, nil
}

// resolveTabFilter finds the filter DSL string for the given tab key.
// If the tab is not found, it returns "all" (no additional filtering).
func resolveTabFilter(tabs []entity.PipelineTab, tabKey string) string {
	if tabKey == "" {
		return filterdsl.FilterAll
	}
	for _, t := range tabs {
		if t.TabKey == tabKey {
			return t.Filter
		}
	}
	return filterdsl.FilterAll
}
