package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

// WorkflowRepository provides data access for workflows and all nested
// canvas/pipeline-config entities.
type WorkflowRepository interface {
	// Workflow CRUD
	List(ctx context.Context, workspaceID string, status *string) ([]entity.WorkflowListItem, error)
	GetByID(ctx context.Context, id string) (*entity.WorkflowFull, error)
	GetBySlug(ctx context.Context, workspaceID, slug string) (*entity.WorkflowFull, error)
	Create(ctx context.Context, w *entity.Workflow) error
	Update(ctx context.Context, id string, fields map[string]interface{}) error
	Delete(ctx context.Context, id string) error

	// Canvas — atomic replace
	SaveCanvas(ctx context.Context, workflowID string, nodes []entity.WorkflowNode, edges []entity.WorkflowEdge) error

	// Pipeline config — atomic replace per entity type
	SaveSteps(ctx context.Context, workflowID string, steps []entity.WorkflowStep) error
	SaveTabs(ctx context.Context, workflowID string, tabs []entity.PipelineTab) error
	SaveStats(ctx context.Context, workflowID string, stats []entity.PipelineStat) error
	SaveColumns(ctx context.Context, workflowID string, cols []entity.PipelineColumn) error
	GetConfig(ctx context.Context, workflowID string) (*entity.WorkflowFull, error)

	// Single step update
	GetStepByKey(ctx context.Context, workflowID, stepKey string) (*entity.WorkflowStep, error)
	UpdateStep(ctx context.Context, workflowID, stepKey string, fields map[string]interface{}) error
}

type workflowRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

// NewWorkflowRepo constructs a WorkflowRepository.
func NewWorkflowRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) WorkflowRepository {
	return &workflowRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *workflowRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

// ─── List ────────────────────────────────────────────────────────────────────

func (r *workflowRepo) List(ctx context.Context, workspaceID string, status *string) ([]entity.WorkflowListItem, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	q := `SELECT w.id::text, w.workspace_id::text, w.name, w.icon, w.slug,
               w.description, w.status, w.stage_filter,
               w.created_by, w.updated_by, w.created_at, w.updated_at,
               COUNT(DISTINCT n.id) AS node_count,
               COUNT(DISTINCT e.id) AS edge_count
        FROM workflows w
        LEFT JOIN workflow_nodes n ON n.workflow_id = w.id
        LEFT JOIN workflow_edges e ON e.workflow_id = w.id
        WHERE w.workspace_id = $1`
	args := []interface{}{workspaceID}

	if status != nil {
		args = append(args, *status)
		q += fmt.Sprintf(" AND w.status = $%d", len(args))
	}
	q += " GROUP BY w.id ORDER BY w.created_at ASC"

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("workflow.List: %w", err)
	}
	defer rows.Close()

	var result []entity.WorkflowListItem
	for rows.Next() {
		var item entity.WorkflowListItem
		var stageFilter pq.StringArray
		if err := rows.Scan(
			&item.ID, &item.WorkspaceID, &item.Name, &item.Icon, &item.Slug,
			&item.Description, &item.Status, &stageFilter,
			&item.CreatedBy, &item.UpdatedBy, &item.CreatedAt, &item.UpdatedAt,
			&item.NodeCount, &item.EdgeCount,
		); err != nil {
			return nil, fmt.Errorf("workflow.List scan: %w", err)
		}
		item.StageFilter = []string(stageFilter)
		result = append(result, item)
	}
	return result, rows.Err()
}

// ─── GetByID ──────────────────────────────────────────────────────────────────

func (r *workflowRepo) GetByID(ctx context.Context, id string) (*entity.WorkflowFull, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	wf, err := r.getWorkflow(ctx, "id", id)
	if err != nil {
		return nil, err
	}
	return r.loadNested(ctx, wf)
}

// ─── GetBySlug ────────────────────────────────────────────────────────────────

func (r *workflowRepo) GetBySlug(ctx context.Context, workspaceID, slug string) (*entity.WorkflowFull, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	row := r.db.QueryRowContext(ctx,
		`SELECT id::text, workspace_id::text, name, icon, slug, description, status,
                stage_filter, created_by, updated_by, created_at, updated_at
         FROM workflows WHERE workspace_id = $1 AND slug = $2`, workspaceID, slug)

	wf, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("workflow.GetBySlug: %w", err)
	}
	return r.loadNested(ctx, wf)
}

// ─── Create ───────────────────────────────────────────────────────────────────

func (r *workflowRepo) Create(ctx context.Context, w *entity.Workflow) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	err := r.db.QueryRowContext(ctx,
		`INSERT INTO workflows (workspace_id, name, icon, slug, description, status, stage_filter, created_by, updated_by)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
         RETURNING id::text, created_at, updated_at`,
		w.WorkspaceID, w.Name, w.Icon, w.Slug, w.Description, w.Status,
		pq.StringArray(w.StageFilter), w.CreatedBy, w.UpdatedBy,
	).Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return fmt.Errorf("workflow.Create: %w", err)
	}
	return nil
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (r *workflowRepo) Update(ctx context.Context, id string, fields map[string]interface{}) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	allowed := map[string]bool{"name": true, "icon": true, "slug": true,
		"description": true, "status": true, "stage_filter": true, "updated_by": true}

	setClauses := make([]string, 0, len(fields))
	args := []interface{}{}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		args = append(args, v)
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, len(args)))
	}
	if len(setClauses) == 0 {
		return nil
	}
	args = append(args, id)
	q := fmt.Sprintf("UPDATE workflows SET %s, updated_at = NOW() WHERE id = $%d",
		strings.Join(setClauses, ", "), len(args))

	_, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("workflow.Update: %w", err)
	}
	return nil
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (r *workflowRepo) Delete(ctx context.Context, id string) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx, `DELETE FROM workflows WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("workflow.Delete: %w", err)
	}
	return nil
}

// ─── SaveCanvas ───────────────────────────────────────────────────────────────

func (r *workflowRepo) SaveCanvas(ctx context.Context, workflowID string, nodes []entity.WorkflowNode, edges []entity.WorkflowEdge) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("workflow.SaveCanvas begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `DELETE FROM workflow_nodes WHERE workflow_id = $1`, workflowID); err != nil {
		return fmt.Errorf("workflow.SaveCanvas delete nodes: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM workflow_edges WHERE workflow_id = $1`, workflowID); err != nil {
		return fmt.Errorf("workflow.SaveCanvas delete edges: %w", err)
	}

	for i := range nodes {
		n := &nodes[i]
		n.WorkflowID = workflowID
		_, err = tx.ExecContext(ctx,
			`INSERT INTO workflow_nodes
             (workflow_id, node_id, node_type, position_x, position_y, width, height, data,
              draggable, selectable, connectable, z_index)
             VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			n.WorkflowID, n.NodeID, n.NodeType, n.PositionX, n.PositionY,
			n.Width, n.Height, []byte(n.Data),
			n.Draggable, n.Selectable, n.Connectable, n.ZIndex,
		)
		if err != nil {
			return fmt.Errorf("workflow.SaveCanvas insert node %s: %w", n.NodeID, err)
		}
	}

	for i := range edges {
		e := &edges[i]
		e.WorkflowID = workflowID
		_, err = tx.ExecContext(ctx,
			`INSERT INTO workflow_edges
             (workflow_id, edge_id, source_node_id, target_node_id, source_handle, target_handle, label, animated, style)
             VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			e.WorkflowID, e.EdgeID, e.SourceNodeID, e.TargetNodeID,
			e.SourceHandle, e.TargetHandle, e.Label, e.Animated, nullableJSON(e.Style),
		)
		if err != nil {
			return fmt.Errorf("workflow.SaveCanvas insert edge %s: %w", e.EdgeID, err)
		}
	}

	if _, err = tx.ExecContext(ctx, `UPDATE workflows SET updated_at = NOW() WHERE id = $1`, workflowID); err != nil {
		return fmt.Errorf("workflow.SaveCanvas update ts: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("workflow.SaveCanvas commit: %w", err)
	}
	return nil
}

// ─── Pipeline config saves ────────────────────────────────────────────────────

func (r *workflowRepo) SaveSteps(ctx context.Context, workflowID string, steps []entity.WorkflowStep) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("workflow.SaveSteps begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `DELETE FROM workflow_steps WHERE workflow_id = $1`, workflowID); err != nil {
		return fmt.Errorf("workflow.SaveSteps delete: %w", err)
	}
	for i := range steps {
		s := &steps[i]
		s.WorkflowID = workflowID
		s.SortOrder = i
		_, err = tx.ExecContext(ctx,
			`INSERT INTO workflow_steps
             (workflow_id, step_key, label, phase, icon, description, sort_order,
              timing, condition, stop_if, sent_flag, template_id, message_template_id, email_template_id)
             VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			s.WorkflowID, s.StepKey, s.Label, s.Phase, s.Icon, s.Description, s.SortOrder,
			s.Timing, s.Condition, s.StopIf, s.SentFlag, s.TemplateID,
			s.MessageTemplateID, s.EmailTemplateID,
		)
		if err != nil {
			return fmt.Errorf("workflow.SaveSteps insert %s: %w", s.StepKey, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("workflow.SaveSteps commit: %w", err)
	}
	return nil
}

func (r *workflowRepo) SaveTabs(ctx context.Context, workflowID string, tabs []entity.PipelineTab) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("workflow.SaveTabs begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `DELETE FROM pipeline_tabs WHERE workflow_id = $1`, workflowID); err != nil {
		return fmt.Errorf("workflow.SaveTabs delete: %w", err)
	}
	for i := range tabs {
		t := &tabs[i]
		t.WorkflowID = workflowID
		t.SortOrder = i
		_, err = tx.ExecContext(ctx,
			`INSERT INTO pipeline_tabs (workflow_id, tab_key, label, icon, filter, sort_order)
             VALUES ($1,$2,$3,$4,$5,$6)`,
			t.WorkflowID, t.TabKey, t.Label, t.Icon, t.Filter, t.SortOrder,
		)
		if err != nil {
			return fmt.Errorf("workflow.SaveTabs insert %s: %w", t.TabKey, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("workflow.SaveTabs commit: %w", err)
	}
	return nil
}

func (r *workflowRepo) SaveStats(ctx context.Context, workflowID string, stats []entity.PipelineStat) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("workflow.SaveStats begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `DELETE FROM pipeline_stats WHERE workflow_id = $1`, workflowID); err != nil {
		return fmt.Errorf("workflow.SaveStats delete: %w", err)
	}
	for i := range stats {
		s := &stats[i]
		s.WorkflowID = workflowID
		s.SortOrder = i
		_, err = tx.ExecContext(ctx,
			`INSERT INTO pipeline_stats (workflow_id, stat_key, label, metric, color, border, sort_order)
             VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			s.WorkflowID, s.StatKey, s.Label, s.Metric, s.Color, s.Border, s.SortOrder,
		)
		if err != nil {
			return fmt.Errorf("workflow.SaveStats insert %s: %w", s.StatKey, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("workflow.SaveStats commit: %w", err)
	}
	return nil
}

func (r *workflowRepo) SaveColumns(ctx context.Context, workflowID string, cols []entity.PipelineColumn) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("workflow.SaveColumns begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `DELETE FROM pipeline_columns WHERE workflow_id = $1`, workflowID); err != nil {
		return fmt.Errorf("workflow.SaveColumns delete: %w", err)
	}
	for i := range cols {
		c := &cols[i]
		c.WorkflowID = workflowID
		c.SortOrder = i
		_, err = tx.ExecContext(ctx,
			`INSERT INTO pipeline_columns (workflow_id, column_key, field, label, width, visible, sort_order)
             VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			c.WorkflowID, c.ColumnKey, c.Field, c.Label, c.Width, c.Visible, c.SortOrder,
		)
		if err != nil {
			return fmt.Errorf("workflow.SaveColumns insert %s: %w", c.ColumnKey, err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("workflow.SaveColumns commit: %w", err)
	}
	return nil
}

// ─── GetConfig ────────────────────────────────────────────────────────────────

func (r *workflowRepo) GetConfig(ctx context.Context, workflowID string) (*entity.WorkflowFull, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	wf, err := r.getWorkflow(ctx, "id", workflowID)
	if err != nil {
		return nil, err
	}
	full := &entity.WorkflowFull{Workflow: *wf}
	if err := r.loadSteps(ctx, full); err != nil {
		return nil, err
	}
	if err := r.loadTabs(ctx, full); err != nil {
		return nil, err
	}
	if err := r.loadStats(ctx, full); err != nil {
		return nil, err
	}
	if err := r.loadColumns(ctx, full); err != nil {
		return nil, err
	}
	return full, nil
}

// ─── Step single update ───────────────────────────────────────────────────────

func (r *workflowRepo) GetStepByKey(ctx context.Context, workflowID, stepKey string) (*entity.WorkflowStep, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	row := r.db.QueryRowContext(ctx,
		`SELECT id::text, workflow_id::text, step_key, label, phase, icon, description, sort_order,
                timing, condition, stop_if, sent_flag, template_id, message_template_id, email_template_id,
                created_at, updated_at
         FROM workflow_steps WHERE workflow_id = $1 AND step_key = $2`, workflowID, stepKey)

	s, err := scanWorkflowStep(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("workflow.GetStepByKey: %w", err)
	}
	return s, nil
}

func (r *workflowRepo) UpdateStep(ctx context.Context, workflowID, stepKey string, fields map[string]interface{}) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	allowed := map[string]bool{
		"timing": true, "condition": true, "stop_if": true, "sent_flag": true,
		"template_id": true, "message_template_id": true, "email_template_id": true,
		"label": true, "description": true, "icon": true,
	}
	setClauses := make([]string, 0)
	args := []interface{}{}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		args = append(args, v)
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, len(args)))
	}
	if len(setClauses) == 0 {
		return nil
	}
	args = append(args, workflowID, stepKey)
	q := fmt.Sprintf("UPDATE workflow_steps SET %s, updated_at = NOW() WHERE workflow_id = $%d AND step_key = $%d",
		strings.Join(setClauses, ", "), len(args)-1, len(args))
	_, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("workflow.UpdateStep: %w", err)
	}
	return nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (r *workflowRepo) getWorkflow(ctx context.Context, col, val string) (*entity.Workflow, error) {
	row := r.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id::text, workspace_id::text, name, icon, slug, description, status,
                stage_filter, created_by, updated_by, created_at, updated_at
         FROM workflows WHERE %s = $1`, col), val)
	wf, err := scanWorkflow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("workflow.getWorkflow(%s): %w", col, err)
	}
	return wf, nil
}

func (r *workflowRepo) loadNested(ctx context.Context, wf *entity.Workflow) (*entity.WorkflowFull, error) {
	if wf == nil {
		return nil, nil
	}
	full := &entity.WorkflowFull{Workflow: *wf}
	if err := r.loadNodes(ctx, full); err != nil {
		return nil, err
	}
	if err := r.loadEdges(ctx, full); err != nil {
		return nil, err
	}
	if err := r.loadSteps(ctx, full); err != nil {
		return nil, err
	}
	if err := r.loadTabs(ctx, full); err != nil {
		return nil, err
	}
	if err := r.loadStats(ctx, full); err != nil {
		return nil, err
	}
	if err := r.loadColumns(ctx, full); err != nil {
		return nil, err
	}
	return full, nil
}

func (r *workflowRepo) loadNodes(ctx context.Context, full *entity.WorkflowFull) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id::text, workflow_id::text, node_id, node_type, position_x, position_y,
                width, height, data, draggable, selectable, connectable, z_index, created_at, updated_at
         FROM workflow_nodes WHERE workflow_id = $1 ORDER BY z_index, created_at`, full.ID)
	if err != nil {
		return fmt.Errorf("workflow.loadNodes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		n, err := scanWorkflowNode(rows)
		if err != nil {
			return fmt.Errorf("workflow.loadNodes scan: %w", err)
		}
		full.Nodes = append(full.Nodes, *n)
	}
	return rows.Err()
}

func (r *workflowRepo) loadEdges(ctx context.Context, full *entity.WorkflowFull) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id::text, workflow_id::text, edge_id, source_node_id, target_node_id,
                source_handle, target_handle, label, animated, style, created_at
         FROM workflow_edges WHERE workflow_id = $1 ORDER BY created_at`, full.ID)
	if err != nil {
		return fmt.Errorf("workflow.loadEdges: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		e, err := scanWorkflowEdge(rows)
		if err != nil {
			return fmt.Errorf("workflow.loadEdges scan: %w", err)
		}
		full.Edges = append(full.Edges, *e)
	}
	return rows.Err()
}

func (r *workflowRepo) loadSteps(ctx context.Context, full *entity.WorkflowFull) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id::text, workflow_id::text, step_key, label, phase, icon, description, sort_order,
                timing, condition, stop_if, sent_flag, template_id, message_template_id, email_template_id,
                created_at, updated_at
         FROM workflow_steps WHERE workflow_id = $1 ORDER BY sort_order`, full.ID)
	if err != nil {
		return fmt.Errorf("workflow.loadSteps: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		s, err := scanWorkflowStep(rows)
		if err != nil {
			return fmt.Errorf("workflow.loadSteps scan: %w", err)
		}
		full.Steps = append(full.Steps, *s)
	}
	return rows.Err()
}

func (r *workflowRepo) loadTabs(ctx context.Context, full *entity.WorkflowFull) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id::text, workflow_id::text, tab_key, label, icon, filter, sort_order
         FROM pipeline_tabs WHERE workflow_id = $1 ORDER BY sort_order`, full.ID)
	if err != nil {
		return fmt.Errorf("workflow.loadTabs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t entity.PipelineTab
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.TabKey, &t.Label, &t.Icon, &t.Filter, &t.SortOrder); err != nil {
			return fmt.Errorf("workflow.loadTabs scan: %w", err)
		}
		full.Tabs = append(full.Tabs, t)
	}
	return rows.Err()
}

func (r *workflowRepo) loadStats(ctx context.Context, full *entity.WorkflowFull) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id::text, workflow_id::text, stat_key, label, metric, color, border, sort_order
         FROM pipeline_stats WHERE workflow_id = $1 ORDER BY sort_order`, full.ID)
	if err != nil {
		return fmt.Errorf("workflow.loadStats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s entity.PipelineStat
		if err := rows.Scan(&s.ID, &s.WorkflowID, &s.StatKey, &s.Label, &s.Metric, &s.Color, &s.Border, &s.SortOrder); err != nil {
			return fmt.Errorf("workflow.loadStats scan: %w", err)
		}
		full.Stats = append(full.Stats, s)
	}
	return rows.Err()
}

func (r *workflowRepo) loadColumns(ctx context.Context, full *entity.WorkflowFull) error {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id::text, workflow_id::text, column_key, field, label, width, visible, sort_order
         FROM pipeline_columns WHERE workflow_id = $1 ORDER BY sort_order`, full.ID)
	if err != nil {
		return fmt.Errorf("workflow.loadColumns: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c entity.PipelineColumn
		if err := rows.Scan(&c.ID, &c.WorkflowID, &c.ColumnKey, &c.Field, &c.Label, &c.Width, &c.Visible, &c.SortOrder); err != nil {
			return fmt.Errorf("workflow.loadColumns scan: %w", err)
		}
		full.Columns = append(full.Columns, c)
	}
	return rows.Err()
}

// ─── Scanners ─────────────────────────────────────────────────────────────────

func scanWorkflow(s rowScanner) (*entity.Workflow, error) {
	var w entity.Workflow
	var sf pq.StringArray
	err := s.Scan(
		&w.ID, &w.WorkspaceID, &w.Name, &w.Icon, &w.Slug, &w.Description,
		&w.Status, &sf, &w.CreatedBy, &w.UpdatedBy, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	w.StageFilter = []string(sf)
	return &w, nil
}

func scanWorkflowNode(s rowScanner) (*entity.WorkflowNode, error) {
	var n entity.WorkflowNode
	var dataBytes []byte
	err := s.Scan(
		&n.ID, &n.WorkflowID, &n.NodeID, &n.NodeType,
		&n.PositionX, &n.PositionY, &n.Width, &n.Height,
		&dataBytes, &n.Draggable, &n.Selectable, &n.Connectable, &n.ZIndex,
		&n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	n.Data = dataBytes
	return &n, nil
}

func scanWorkflowEdge(s rowScanner) (*entity.WorkflowEdge, error) {
	var e entity.WorkflowEdge
	var styleBytes []byte
	err := s.Scan(
		&e.ID, &e.WorkflowID, &e.EdgeID, &e.SourceNodeID, &e.TargetNodeID,
		&e.SourceHandle, &e.TargetHandle, &e.Label, &e.Animated,
		&styleBytes, &e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	e.Style = styleBytes
	return &e, nil
}

func scanWorkflowStep(s rowScanner) (*entity.WorkflowStep, error) {
	var step entity.WorkflowStep
	err := s.Scan(
		&step.ID, &step.WorkflowID, &step.StepKey, &step.Label, &step.Phase,
		&step.Icon, &step.Description, &step.SortOrder,
		&step.Timing, &step.Condition, &step.StopIf, &step.SentFlag, &step.TemplateID,
		&step.MessageTemplateID, &step.EmailTemplateID,
		&step.CreatedAt, &step.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &step, nil
}

// nullableJSON returns nil if the slice is empty/nil, otherwise the raw bytes.
func nullableJSON(b []byte) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}
