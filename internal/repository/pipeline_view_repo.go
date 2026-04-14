package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/filterdsl"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/pagination"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// PipelineViewRepository provides paginated master_data reads scoped to
// workflow stage filters, tab filters, and stat computation.
type PipelineViewRepository interface {
	ListData(ctx context.Context, req PipelineDataRequest) ([]entity.MasterData, int64, error)
	ComputeStat(ctx context.Context, workspaceID, metric string) (string, error)
}

// PipelineDataRequest carries all parameters for a pipeline data query.
type PipelineDataRequest struct {
	WorkspaceID  string
	StageFilter  []string // from workflow.stage_filter
	TabFilter    string   // DSL filter string from the active pipeline tab
	Search       string
	Pagination   pagination.Params
	SortBy       string
	SortDir      string
}

type pipelineViewRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewPipelineViewRepo constructs a PipelineViewRepository.
func NewPipelineViewRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) PipelineViewRepository {
	return &pipelineViewRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *pipelineViewRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// pipelineBuildPlaceholders appends string values to args and returns
// comma-separated positional placeholders ($N,...) for use in SQL IN clauses.
func pipelineBuildPlaceholders(existing []interface{}, vals []string) (string, []interface{}) {
	args := make([]interface{}, len(existing), len(existing)+len(vals))
	copy(args, existing)
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		args = append(args, v)
		placeholders[i] = fmt.Sprintf("$%d", len(args))
	}
	return strings.Join(placeholders, ","), args
}

var allowedSortCols = map[string]bool{
	"updated_at": true, "company_name": true, "stage": true,
	"created_at": true, "contract_end": true, "final_price": true,
	"days_to_expiry": true,
}

// ListData returns master_data records filtered by workflow stage + tab DSL.
func (r *pipelineViewRepo) ListData(ctx context.Context, req PipelineDataRequest) ([]entity.MasterData, int64, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	// Build base WHERE from tab filter DSL
	where, args := filterdsl.ParseFilter(req.TabFilter, req.WorkspaceID)

	// Additional stage filter from workflow
	if len(req.StageFilter) > 0 {
		placeholders, newArgs := pipelineBuildPlaceholders(args, req.StageFilter)
		where += " AND stage IN (" + placeholders + ")"
		args = newArgs
	}

	// Search
	if req.Search != "" {
		args = append(args, "%"+req.Search+"%")
		n := len(args)
		where += fmt.Sprintf(" AND (company_name ILIKE $%d OR company_id ILIKE $%d OR pic_name ILIKE $%d)", n, n, n)
	}

	// Count
	var total int64
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM master_data WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("pipelineView.ListData count: %w", err)
	}

	// Sort
	sortCol := "updated_at"
	if allowedSortCols[req.SortBy] {
		sortCol = req.SortBy
	}
	sortDir := "DESC"
	if req.SortDir == "asc" {
		sortDir = "ASC"
	}

	// Paginate
	limit := req.Pagination.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	offset := req.Pagination.Offset

	args = append(args, limit, offset)
	q := fmt.Sprintf(
		`SELECT id::text, workspace_id::text, company_id, company_name, stage,
                pic_name, pic_nickname, pic_role, pic_wa, pic_email,
                owner_name, owner_wa, owner_telegram_id,
                bot_active, blacklisted, sequence_status, snooze_until, snooze_reason,
                risk_flag, contract_start, contract_end, contract_months, days_to_expiry,
                payment_status, payment_terms, final_price,
                custom_fields, created_at, updated_at
         FROM master_data
         WHERE %s
         ORDER BY %s %s
         LIMIT $%d OFFSET $%d`,
		where, sortCol, sortDir, len(args)-1, len(args),
	)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("pipelineView.ListData query: %w", err)
	}
	defer rows.Close()

	var result []entity.MasterData
	for rows.Next() {
		var m entity.MasterData
		var cfBytes []byte
		if err := rows.Scan(
			&m.ID, &m.WorkspaceID, &m.CompanyID, &m.CompanyName, &m.Stage,
			&m.PICName, &m.PICNickname, &m.PICRole, &m.PICWA, &m.PICEmail,
			&m.OwnerName, &m.OwnerWA, &m.OwnerTelegramID,
			&m.BotActive, &m.Blacklisted, &m.SequenceStatus, &m.SnoozeUntil, &m.SnoozeReason,
			&m.RiskFlag, &m.ContractStart, &m.ContractEnd, &m.ContractMonths, &m.DaysToExpiry,
			&m.PaymentStatus, &m.PaymentTerms, &m.FinalPrice,
			&cfBytes, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("pipelineView.ListData scan: %w", err)
		}
		if len(cfBytes) > 0 {
			_ = json.Unmarshal(cfBytes, &m.CustomFields)
		}
		result = append(result, m)
	}
	return result, total, rows.Err()
}

// ComputeStat executes a metric DSL query and returns the scalar result as string.
func (r *pipelineViewRepo) ComputeStat(ctx context.Context, workspaceID, metric string) (string, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q, args := filterdsl.ComputeMetricQuery(metric, workspaceID)
	if args == nil {
		return "0", nil
	}
	var val float64
	if err := r.db.QueryRowContext(ctx, q, args...).Scan(&val); err != nil {
		return "0", fmt.Errorf("pipelineView.ComputeStat(%s): %w", metric, err)
	}
	if val == float64(int64(val)) {
		return strconv.FormatInt(int64(val), 10), nil
	}
	return strconv.FormatFloat(val, 'f', 2, 64), nil
}

