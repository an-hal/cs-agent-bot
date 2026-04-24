// Package rejectionanalysis classifies + stores rejection replies so coaching
// and the analytics dashboard can surface common objection patterns.
//
// Rule-based classifier is implemented inline; a future Claude-backed analyst
// can be added by replacing Analyze with an external call.
package rejectionanalysis

import (
	"context"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

type Usecase interface {
	Analyze(ctx context.Context, req AnalyzeRequest) (*entity.RejectionAnalysis, error)
	Record(ctx context.Context, req RecordRequest) (*entity.RejectionAnalysis, error)
	List(ctx context.Context, filter entity.RejectionAnalysisFilter) ([]entity.RejectionAnalysis, int64, error)
	CategoryStats(ctx context.Context, workspaceID string, sinceDays int) (map[string]int, error)
}

// AnalyzeRequest runs the rule classifier over a reply and stores the result.
type AnalyzeRequest struct {
	WorkspaceID   string `json:"workspace_id"`
	MasterDataID  string `json:"master_data_id"`
	SourceChannel string `json:"source_channel"`
	SourceMessage string `json:"source_message"`
}

// RecordRequest stores a pre-classified rejection (used when analyst=human or
// analyst=claude from an external pipeline).
type RecordRequest struct {
	WorkspaceID       string `json:"workspace_id"`
	MasterDataID      string `json:"master_data_id"`
	SourceChannel     string `json:"source_channel"`
	SourceMessage     string `json:"source_message"`
	RejectionCategory string `json:"rejection_category"`
	Severity          string `json:"severity"`
	AnalysisSummary   string `json:"analysis_summary"`
	SuggestedResponse string `json:"suggested_response"`
	Analyst           string `json:"analyst"`
	AnalystVersion    string `json:"analyst_version"`
}

type usecase struct {
	repo repository.RejectionAnalysisRepository
}

func New(repo repository.RejectionAnalysisRepository) Usecase {
	return &usecase{repo: repo}
}

func (u *usecase) Analyze(ctx context.Context, req AnalyzeRequest) (*entity.RejectionAnalysis, error) {
	if req.WorkspaceID == "" || req.MasterDataID == "" {
		return nil, apperror.ValidationError("workspace_id and master_data_id required")
	}
	cat, sev := classifyRule(req.SourceMessage)
	return u.repo.Insert(ctx, &entity.RejectionAnalysis{
		WorkspaceID:       req.WorkspaceID,
		MasterDataID:      req.MasterDataID,
		SourceChannel:     defaultString(req.SourceChannel, "wa"),
		SourceMessage:     req.SourceMessage,
		RejectionCategory: cat,
		Severity:          sev,
		Analyst:           entity.RejectionAnalystRule,
		AnalystVersion:    "rule-v1",
	})
}

func (u *usecase) Record(ctx context.Context, req RecordRequest) (*entity.RejectionAnalysis, error) {
	if req.WorkspaceID == "" || req.MasterDataID == "" {
		return nil, apperror.ValidationError("workspace_id and master_data_id required")
	}
	if req.RejectionCategory == "" {
		return nil, apperror.ValidationError("rejection_category required when recording externally-classified analysis")
	}
	return u.repo.Insert(ctx, &entity.RejectionAnalysis{
		WorkspaceID:       req.WorkspaceID,
		MasterDataID:      req.MasterDataID,
		SourceChannel:     defaultString(req.SourceChannel, "wa"),
		SourceMessage:     req.SourceMessage,
		RejectionCategory: req.RejectionCategory,
		Severity:          defaultString(req.Severity, entity.RejectionSeverityMid),
		AnalysisSummary:   req.AnalysisSummary,
		SuggestedResponse: req.SuggestedResponse,
		Analyst:           defaultString(req.Analyst, entity.RejectionAnalystHuman),
		AnalystVersion:    req.AnalystVersion,
	})
}

func (u *usecase) List(ctx context.Context, f entity.RejectionAnalysisFilter) ([]entity.RejectionAnalysis, int64, error) {
	if f.WorkspaceID == "" {
		return nil, 0, apperror.ValidationError("workspace_id required")
	}
	return u.repo.List(ctx, f)
}

func (u *usecase) CategoryStats(ctx context.Context, workspaceID string, sinceDays int) (map[string]int, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if sinceDays <= 0 {
		sinceDays = 30
	}
	since := nowUTC().AddDate(0, 0, -sinceDays)
	return u.repo.CountByCategory(ctx, workspaceID, since)
}

// classifyRule is a very small keyword classifier; good enough to surface
// patterns in the dashboard until a Claude-backed analyst is wired in.
func classifyRule(message string) (category, severity string) {
	m := strings.ToLower(message)
	switch {
	case containsAny(m, "mahal", "harga", "budget", "biaya", "diskon"):
		return entity.RejectionCategoryPrice, entity.RejectionSeverityMid
	case containsAny(m, "bos", "atasan", "cfo", "ceo", "direksi", "tanya atasan"):
		return entity.RejectionCategoryAuthority, entity.RejectionSeverityMid
	case containsAny(m, "nanti", "bulan depan", "quarter", "q3", "q4", "masih lama", "belum butuh"):
		return entity.RejectionCategoryTiming, entity.RejectionSeverityMid
	case containsAny(m, "fitur", "integrasi", "api", "laporan", "belum ada", "kurang"):
		return entity.RejectionCategoryFeature, entity.RejectionSeverityMid
	case containsAny(m, "ga enak", "marah", "komplain", "kecewa", "jelek"):
		return entity.RejectionCategoryTone, entity.RejectionSeverityHigh
	}
	return entity.RejectionCategoryOther, entity.RejectionSeverityLow
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func defaultString(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
