# Golang Models — Workflow Engine

## Models

```go
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ══════════════════════════════════════════════════════════════
// Workflow — top-level pipeline definition
// ══════════════════════════════════════════════════════════════

type WorkflowStatus string

const (
	WorkflowActive   WorkflowStatus = "active"
	WorkflowDraft    WorkflowStatus = "draft"
	WorkflowDisabled WorkflowStatus = "disabled"
)

type Workflow struct {
	ID          uuid.UUID      `db:"id" json:"id"`
	WorkspaceID uuid.UUID     `db:"workspace_id" json:"workspace_id"`

	Name        string         `db:"name" json:"name"`
	Icon        *string        `db:"icon" json:"icon"`
	Slug        string         `db:"slug" json:"slug"`
	Description *string        `db:"description" json:"description"`
	Status      WorkflowStatus `db:"status" json:"status"`

	// Stage filter — which master_data stages this workflow processes
	// PostgreSQL text[] → Go []string via pq.StringArray
	StageFilter []string `db:"stage_filter" json:"stage_filter"`

	CreatedBy *string   `db:"created_by" json:"created_by"`
	UpdatedBy *string   `db:"updated_by" json:"updated_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// WorkflowFull includes all nested entities for the builder/pipeline view
type WorkflowFull struct {
	Workflow
	Nodes   []WorkflowNode   `json:"nodes"`
	Edges   []WorkflowEdge   `json:"edges"`
	Steps   []WorkflowStep   `json:"steps"`
	Tabs    []PipelineTab    `json:"tabs"`
	Stats   []PipelineStat   `json:"stats"`
	Columns []PipelineColumn `json:"columns"`
}

// ══════════════════════════════════════════════════════════════
// WorkflowNode — canvas node (React Flow)
// ══════════════════════════════════════════════════════════════

type NodeCategory string

const (
	NodeTrigger   NodeCategory = "trigger"
	NodeCondition NodeCategory = "condition"
	NodeAction    NodeCategory = "action"
	NodeDelay     NodeCategory = "delay"
)

type WorkflowNode struct {
	ID         uuid.UUID `db:"id" json:"id"`
	WorkflowID uuid.UUID `db:"workflow_id" json:"workflow_id"`

	NodeID   string `db:"node_id" json:"node_id"`     // e.g. 'ae-p01', 'sdr-wa-h3'
	NodeType string `db:"node_type" json:"node_type"` // 'workflow' | 'zone'

	// Position
	PositionX float64 `db:"position_x" json:"position_x"`
	PositionY float64 `db:"position_y" json:"position_y"`

	// Dimensions (zone nodes)
	Width  *float64 `db:"width" json:"width,omitempty"`
	Height *float64 `db:"height" json:"height,omitempty"`

	// Node data — all config in JSONB
	// Contains: category, label, icon, description, templateId, triggerId,
	//           timing, condition, stopIf, sentFlag
	Data json.RawMessage `db:"data" json:"data"`

	// Display flags
	Draggable   bool `db:"draggable" json:"draggable"`
	Selectable  bool `db:"selectable" json:"selectable"`
	Connectable bool `db:"connectable" json:"connectable"`
	ZIndex      int  `db:"z_index" json:"z_index"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// NodeData is the typed version of the JSONB data field
type NodeData struct {
	Category    NodeCategory `json:"category"`
	Label       string       `json:"label"`
	Icon        string       `json:"icon,omitempty"`
	Description string       `json:"description,omitempty"`
	TemplateID  string       `json:"templateId,omitempty"`
	TriggerID   string       `json:"triggerId,omitempty"`
	Timing      string       `json:"timing,omitempty"`
	Condition   string       `json:"condition,omitempty"`
	StopIf      string       `json:"stopIf,omitempty"`
	SentFlag    string       `json:"sentFlag,omitempty"`

	// Zone-only fields
	Color string `json:"color,omitempty"`
	Bg    string `json:"bg,omitempty"`
}

// GetNodeData parses the JSONB data field into a typed struct
func (n *WorkflowNode) GetNodeData() (*NodeData, error) {
	var d NodeData
	if err := json.Unmarshal(n.Data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// ══════════════════════════════════════════════════════════════
// WorkflowEdge — canvas edge (React Flow)
// ══════════════════════════════════════════════════════════════

type WorkflowEdge struct {
	ID         uuid.UUID `db:"id" json:"id"`
	WorkflowID uuid.UUID `db:"workflow_id" json:"workflow_id"`

	EdgeID       string  `db:"edge_id" json:"edge_id"`
	SourceNodeID string  `db:"source_node_id" json:"source"`
	TargetNodeID string  `db:"target_node_id" json:"target"`
	SourceHandle *string `db:"source_handle" json:"sourceHandle,omitempty"`
	TargetHandle *string `db:"target_handle" json:"targetHandle,omitempty"`

	Label    *string         `db:"label" json:"label,omitempty"`
	Animated bool            `db:"animated" json:"animated"`
	Style    json.RawMessage `db:"style" json:"style,omitempty"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ══════════════════════════════════════════════════════════════
// WorkflowStep — pipeline step configuration
// ══════════════════════════════════════════════════════════════

type WorkflowStep struct {
	ID         uuid.UUID `db:"id" json:"id"`
	WorkflowID uuid.UUID `db:"workflow_id" json:"workflow_id"`

	StepKey     string  `db:"step_key" json:"key"`
	Label       string  `db:"label" json:"label"`
	Phase       string  `db:"phase" json:"phase"`
	Icon        *string `db:"icon" json:"icon"`
	Description *string `db:"description" json:"description"`
	SortOrder   int     `db:"sort_order" json:"sort_order"`

	// Automation config
	Timing    *string `db:"timing" json:"timing"`
	Condition *string `db:"condition" json:"condition"`
	StopIf    *string `db:"stop_if" json:"stopIf"`
	SentFlag  *string `db:"sent_flag" json:"sentFlag"`
	TemplateID *string `db:"template_id" json:"templateId"`

	// Template library references
	MessageTemplateID *string `db:"message_template_id" json:"messageTemplateId"`
	EmailTemplateID   *string `db:"email_template_id" json:"emailTemplateId"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ══════════════════════════════════════════════════════════════
// AutomationRule — executable rule (evaluated by cron/events)
// ══════════════════════════════════════════════════════════════

type RuleStatus string

const (
	RuleActive   RuleStatus = "active"
	RulePaused   RuleStatus = "paused"
	RuleDisabled RuleStatus = "disabled"
)

type RuleChannel string

const (
	ChannelWhatsApp RuleChannel = "whatsapp"
	ChannelEmail    RuleChannel = "email"
	ChannelTelegram RuleChannel = "telegram"
)

type RuleRole string

const (
	RoleSDR RuleRole = "sdr"
	RoleBD  RuleRole = "bd"
	RoleAE  RuleRole = "ae"
	RoleCS  RuleRole = "cs"
)

type AutomationRule struct {
	ID          uuid.UUID `db:"id" json:"id"`
	WorkspaceID uuid.UUID `db:"workspace_id" json:"workspace_id"`

	RuleCode   string `db:"rule_code" json:"rule_code"`     // e.g. 'RULE-KK-AE-OB-001'
	TriggerID  string `db:"trigger_id" json:"trigger_id"`   // e.g. 'Onboarding_Welcome'
	TemplateID *string `db:"template_id" json:"template_id"` // message template ref

	// Classification
	Role       RuleRole `db:"role" json:"role"`
	Phase      string   `db:"phase" json:"phase"`
	PhaseLabel *string  `db:"phase_label" json:"phase_label"`
	Priority   *string  `db:"priority" json:"priority"`

	// Execution config
	Timing    string      `db:"timing" json:"timing"`
	Condition string      `db:"condition" json:"condition"`
	StopIf    string      `db:"stop_if" json:"stop_if"`
	SentFlag  *string     `db:"sent_flag" json:"sent_flag"`
	Channel   RuleChannel `db:"channel" json:"channel"`

	// State
	Status RuleStatus `db:"status" json:"status"`

	// Meta
	UpdatedAt *time.Time `db:"updated_at" json:"updated_at"`
	UpdatedBy *string    `db:"updated_by" json:"updated_by"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}

// IsExecutable checks if the rule can be evaluated by the cron engine
func (r *AutomationRule) IsExecutable() bool {
	return r.Status == RuleActive
}

// ══════════════════════════════════════════════════════════════
// RuleChangeLog — audit trail
// ══════════════════════════════════════════════════════════════

type RuleChangeLog struct {
	ID          uuid.UUID `db:"id" json:"id"`
	RuleID      uuid.UUID `db:"rule_id" json:"rule_id"`
	WorkspaceID uuid.UUID `db:"workspace_id" json:"workspace_id"`

	Field    string  `db:"field" json:"field"`         // 'timing', 'condition', 'status', 'created'
	OldValue *string `db:"old_value" json:"old_value"`
	NewValue string  `db:"new_value" json:"new_value"`

	EditedBy string    `db:"edited_by" json:"edited_by"`
	EditedAt time.Time `db:"edited_at" json:"edited_at"`
}

// ══════════════════════════════════════════════════════════════
// Pipeline Config — tabs, stats, columns
// ══════════════════════════════════════════════════════════════

type PipelineTab struct {
	ID         uuid.UUID `db:"id" json:"id"`
	WorkflowID uuid.UUID `db:"workflow_id" json:"workflow_id"`

	TabKey    string  `db:"tab_key" json:"key"`
	Label     string  `db:"label" json:"label"`
	Icon      *string `db:"icon" json:"icon"`
	Filter    string  `db:"filter" json:"filter"`     // filter DSL string
	SortOrder int     `db:"sort_order" json:"sort_order"`
}

type PipelineStat struct {
	ID         uuid.UUID `db:"id" json:"id"`
	WorkflowID uuid.UUID `db:"workflow_id" json:"workflow_id"`

	StatKey   string  `db:"stat_key" json:"key"`
	Label     string  `db:"label" json:"label"`
	Metric    string  `db:"metric" json:"metric"`     // metric DSL string
	Color     *string `db:"color" json:"color"`       // tailwind text class
	Border    *string `db:"border" json:"border"`     // tailwind border class
	SortOrder int     `db:"sort_order" json:"sort_order"`
}

type PipelineColumn struct {
	ID         uuid.UUID `db:"id" json:"id"`
	WorkflowID uuid.UUID `db:"workflow_id" json:"workflow_id"`

	ColumnKey string `db:"column_key" json:"key"`
	Field     string `db:"field" json:"field"`       // master_data field name
	Label     string `db:"label" json:"label"`
	Width     int    `db:"width" json:"width"`
	Visible   bool   `db:"visible" json:"visible"`
	SortOrder int    `db:"sort_order" json:"sort_order"`
}

// ══════════════════════════════════════════════════════════════
// NodeConfig — reference data from Excel specs
// Used by the config panel to display pre-populated spec data
// ══════════════════════════════════════════════════════════════

type NodeConfig struct {
	TriggerID  string `json:"triggerId"`
	Phase      string `json:"phase"`
	Action     string `json:"action"`
	Timing     string `json:"timing"`
	Condition  string `json:"condition"`
	TemplateID string `json:"templateId"`
	WAMessage  string `json:"waMessage"`
	Variables  string `json:"variables"`
	StopIf     string `json:"stopIf"`
	SentFlag   string `json:"sentFlag"`
	Notes      string `json:"notes"`
	DataRead   string `json:"dataRead,omitempty"`
	DataWrite  string `json:"dataWrite,omitempty"`
}
```

## Repository Pattern

```go
package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ══════════════════════════════════════════════════════════════
// WorkflowRepo
// ══════════════════════════════════════════════════════════════

type WorkflowRepo struct {
	db *sqlx.DB
}

func NewWorkflowRepo(db *sqlx.DB) *WorkflowRepo {
	return &WorkflowRepo{db: db}
}

// ── List workflows for workspace ────────────────────────────

func (r *WorkflowRepo) ListByWorkspace(ctx context.Context, wsID uuid.UUID, status *string) ([]Workflow, error) {
	query := `SELECT * FROM workflows WHERE workspace_id = $1`
	args := []interface{}{wsID}

	if status != nil {
		query += ` AND status = $2`
		args = append(args, *status)
	}

	query += ` ORDER BY created_at ASC`

	var results []Workflow
	err := r.db.SelectContext(ctx, &results, query, args...)
	return results, err
}

// ── Get workflow with all nested entities ────────────────────

func (r *WorkflowRepo) GetFull(ctx context.Context, id uuid.UUID) (*WorkflowFull, error) {
	var wf Workflow
	err := r.db.GetContext(ctx, &wf, `SELECT * FROM workflows WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}

	full := &WorkflowFull{Workflow: wf}

	r.db.SelectContext(ctx, &full.Nodes, `SELECT * FROM workflow_nodes WHERE workflow_id = $1 ORDER BY z_index, created_at`, id)
	r.db.SelectContext(ctx, &full.Edges, `SELECT * FROM workflow_edges WHERE workflow_id = $1 ORDER BY created_at`, id)
	r.db.SelectContext(ctx, &full.Steps, `SELECT * FROM workflow_steps WHERE workflow_id = $1 ORDER BY sort_order`, id)
	r.db.SelectContext(ctx, &full.Tabs, `SELECT * FROM pipeline_tabs WHERE workflow_id = $1 ORDER BY sort_order`, id)
	r.db.SelectContext(ctx, &full.Stats, `SELECT * FROM pipeline_stats WHERE workflow_id = $1 ORDER BY sort_order`, id)
	r.db.SelectContext(ctx, &full.Columns, `SELECT * FROM pipeline_columns WHERE workflow_id = $1 ORDER BY sort_order`, id)

	return full, nil
}

// ── Get workflow by slug ─────────────────────────────────────

func (r *WorkflowRepo) GetBySlug(ctx context.Context, wsID uuid.UUID, slug string) (*WorkflowFull, error) {
	var wf Workflow
	err := r.db.GetContext(ctx, &wf, `SELECT * FROM workflows WHERE workspace_id = $1 AND slug = $2`, wsID, slug)
	if err != nil {
		return nil, err
	}
	return r.GetFull(ctx, wf.ID)
}

// ── Create workflow ──────────────────────────────────────────

func (r *WorkflowRepo) Create(ctx context.Context, wf *Workflow) error {
	query := `INSERT INTO workflows (workspace_id, name, icon, slug, description, status, stage_filter, created_by, updated_by)
		VALUES (:workspace_id, :name, :icon, :slug, :description, :status, :stage_filter, :created_by, :updated_by)
		RETURNING id, created_at, updated_at`

	rows, err := r.db.NamedQueryContext(ctx, query, wf)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		rows.Scan(&wf.ID, &wf.CreatedAt, &wf.UpdatedAt)
	}
	return nil
}

// ── Save canvas (nodes + edges) — bulk replace ──────────────

func (r *WorkflowRepo) SaveCanvas(ctx context.Context, workflowID uuid.UUID, nodes []WorkflowNode, edges []WorkflowEdge) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing nodes and edges
	tx.ExecContext(ctx, `DELETE FROM workflow_nodes WHERE workflow_id = $1`, workflowID)
	tx.ExecContext(ctx, `DELETE FROM workflow_edges WHERE workflow_id = $1`, workflowID)

	// Insert new nodes
	for _, n := range nodes {
		n.WorkflowID = workflowID
		if n.ID == uuid.Nil {
			n.ID = uuid.New()
		}
		_, err = tx.NamedExecContext(ctx, `INSERT INTO workflow_nodes
			(id, workflow_id, node_id, node_type, position_x, position_y, width, height, data, draggable, selectable, connectable, z_index)
			VALUES (:id, :workflow_id, :node_id, :node_type, :position_x, :position_y, :width, :height, :data, :draggable, :selectable, :connectable, :z_index)`, n)
		if err != nil {
			return fmt.Errorf("insert node %s: %w", n.NodeID, err)
		}
	}

	// Insert new edges
	for _, e := range edges {
		e.WorkflowID = workflowID
		if e.ID == uuid.Nil {
			e.ID = uuid.New()
		}
		_, err = tx.NamedExecContext(ctx, `INSERT INTO workflow_edges
			(id, workflow_id, edge_id, source_node_id, target_node_id, source_handle, target_handle, label, animated, style)
			VALUES (:id, :workflow_id, :edge_id, :source_node_id, :target_node_id, :source_handle, :target_handle, :label, :animated, :style)`, e)
		if err != nil {
			return fmt.Errorf("insert edge %s: %w", e.EdgeID, err)
		}
	}

	// Update workflow updated_at
	tx.ExecContext(ctx, `UPDATE workflows SET updated_at = NOW() WHERE id = $1`, workflowID)

	return tx.Commit()
}

// ── Delete workflow (cascades nodes, edges, steps, tabs, stats, columns) ──

func (r *WorkflowRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM workflows WHERE id = $1`, id)
	return err
}

// ══════════════════════════════════════════════════════════════
// AutomationRuleRepo
// ══════════════════════════════════════════════════════════════

type AutomationRuleRepo struct {
	db *sqlx.DB
}

func NewAutomationRuleRepo(db *sqlx.DB) *AutomationRuleRepo {
	return &AutomationRuleRepo{db: db}
}

func (r *AutomationRuleRepo) ListByWorkspace(ctx context.Context, wsID uuid.UUID, role *string, status *string) ([]AutomationRule, error) {
	query := `SELECT * FROM automation_rules WHERE workspace_id = $1`
	args := []interface{}{wsID}
	argIdx := 2

	if role != nil {
		query += fmt.Sprintf(` AND role = $%d`, argIdx)
		args = append(args, *role)
		argIdx++
	}
	if status != nil {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *status)
		argIdx++
	}

	query += ` ORDER BY phase, created_at`

	var results []AutomationRule
	err := r.db.SelectContext(ctx, &results, query, args...)
	return results, err
}

func (r *AutomationRuleRepo) GetByTriggerID(ctx context.Context, wsID uuid.UUID, triggerID string) (*AutomationRule, error) {
	var rule AutomationRule
	err := r.db.GetContext(ctx, &rule,
		`SELECT * FROM automation_rules WHERE workspace_id = $1 AND trigger_id = $2 AND status = 'active'`,
		wsID, triggerID)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *AutomationRuleRepo) GetActiveByRole(ctx context.Context, wsID uuid.UUID, role RuleRole) ([]AutomationRule, error) {
	var results []AutomationRule
	err := r.db.SelectContext(ctx, &results,
		`SELECT * FROM automation_rules WHERE workspace_id = $1 AND role = $2 AND status = 'active' ORDER BY phase, created_at`,
		wsID, role)
	return results, err
}

func (r *AutomationRuleRepo) Create(ctx context.Context, rule *AutomationRule) error {
	query := `INSERT INTO automation_rules
		(workspace_id, rule_code, trigger_id, template_id, role, phase, phase_label, priority,
		 timing, condition, stop_if, sent_flag, channel, status, updated_by)
		VALUES
		(:workspace_id, :rule_code, :trigger_id, :template_id, :role, :phase, :phase_label, :priority,
		 :timing, :condition, :stop_if, :sent_flag, :channel, :status, :updated_by)
		RETURNING id, created_at`

	rows, err := r.db.NamedQueryContext(ctx, query, rule)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		rows.Scan(&rule.ID, &rule.CreatedAt)
	}
	return nil
}

func (r *AutomationRuleRepo) Update(ctx context.Context, id uuid.UUID, updates map[string]interface{}, editor string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get current rule for change log
	var current AutomationRule
	tx.GetContext(ctx, &current, `SELECT * FROM automation_rules WHERE id = $1`, id)

	// Apply updates
	for field, value := range updates {
		_, err = tx.ExecContext(ctx,
			fmt.Sprintf(`UPDATE automation_rules SET %s = $1, updated_at = NOW(), updated_by = $2 WHERE id = $3`, field),
			value, editor, id)
		if err != nil {
			return err
		}

		// Log change
		var oldVal string
		switch field {
		case "timing":
			oldVal = current.Timing
		case "condition":
			oldVal = current.Condition
		case "stop_if":
			oldVal = current.StopIf
		case "status":
			oldVal = string(current.Status)
		}

		tx.ExecContext(ctx, `INSERT INTO rule_change_logs (rule_id, workspace_id, field, old_value, new_value, edited_by)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			id, current.WorkspaceID, field, oldVal, fmt.Sprint(value), editor)
	}

	return tx.Commit()
}

func (r *AutomationRuleRepo) GetChangeLogs(ctx context.Context, wsID uuid.UUID, limit int) ([]RuleChangeLog, error) {
	var logs []RuleChangeLog
	err := r.db.SelectContext(ctx, &logs,
		`SELECT * FROM rule_change_logs WHERE workspace_id = $1 ORDER BY edited_at DESC LIMIT $2`,
		wsID, limit)
	return logs, err
}

// ══════════════════════════════════════════════════════════════
// Pipeline Config Repos (tabs, stats, columns)
// ══════════════════════════════════════════════════════════════

type PipelineConfigRepo struct {
	db *sqlx.DB
}

func NewPipelineConfigRepo(db *sqlx.DB) *PipelineConfigRepo {
	return &PipelineConfigRepo{db: db}
}

// ── Tabs ────────────────────────────────────────────────────

func (r *PipelineConfigRepo) SaveTabs(ctx context.Context, workflowID uuid.UUID, tabs []PipelineTab) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.ExecContext(ctx, `DELETE FROM pipeline_tabs WHERE workflow_id = $1`, workflowID)
	for i, t := range tabs {
		t.WorkflowID = workflowID
		t.SortOrder = i
		if t.ID == uuid.Nil {
			t.ID = uuid.New()
		}
		_, err = tx.NamedExecContext(ctx, `INSERT INTO pipeline_tabs
			(id, workflow_id, tab_key, label, icon, filter, sort_order)
			VALUES (:id, :workflow_id, :tab_key, :label, :icon, :filter, :sort_order)`, t)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── Stats ───────────────────────────────────────────────────

func (r *PipelineConfigRepo) SaveStats(ctx context.Context, workflowID uuid.UUID, stats []PipelineStat) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.ExecContext(ctx, `DELETE FROM pipeline_stats WHERE workflow_id = $1`, workflowID)
	for i, s := range stats {
		s.WorkflowID = workflowID
		s.SortOrder = i
		if s.ID == uuid.Nil {
			s.ID = uuid.New()
		}
		_, err = tx.NamedExecContext(ctx, `INSERT INTO pipeline_stats
			(id, workflow_id, stat_key, label, metric, color, border, sort_order)
			VALUES (:id, :workflow_id, :stat_key, :label, :metric, :color, :border, :sort_order)`, s)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── Columns ─────────────────────────────────────────────────

func (r *PipelineConfigRepo) SaveColumns(ctx context.Context, workflowID uuid.UUID, cols []PipelineColumn) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.ExecContext(ctx, `DELETE FROM pipeline_columns WHERE workflow_id = $1`, workflowID)
	for i, c := range cols {
		c.WorkflowID = workflowID
		c.SortOrder = i
		if c.ID == uuid.Nil {
			c.ID = uuid.New()
		}
		_, err = tx.NamedExecContext(ctx, `INSERT INTO pipeline_columns
			(id, workflow_id, column_key, field, label, width, visible, sort_order)
			VALUES (:id, :workflow_id, :column_key, :field, :label, :width, :visible, :sort_order)`, c)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── Steps ───────────────────────────────────────────────────

func (r *PipelineConfigRepo) SaveSteps(ctx context.Context, workflowID uuid.UUID, steps []WorkflowStep) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.ExecContext(ctx, `DELETE FROM workflow_steps WHERE workflow_id = $1`, workflowID)
	for i, s := range steps {
		s.WorkflowID = workflowID
		s.SortOrder = i
		if s.ID == uuid.Nil {
			s.ID = uuid.New()
		}
		_, err = tx.NamedExecContext(ctx, `INSERT INTO workflow_steps
			(id, workflow_id, step_key, label, phase, icon, description, sort_order,
			 timing, condition, stop_if, sent_flag, template_id, message_template_id, email_template_id)
			VALUES (:id, :workflow_id, :step_key, :label, :phase, :icon, :description, :sort_order,
			 :timing, :condition, :stop_if, :sent_flag, :template_id, :message_template_id, :email_template_id)`, s)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
```

## Helpers: Working Day & Timing

```go
package helpers

import (
	"time"
)

// Indonesian national holidays (update yearly or load from DB/config)
var nationalHolidays = map[string]bool{
	// Format: "01-02" (MM-DD) or "2026-04-10" (full date for moving holidays)
}

// IsWorkingDay checks if a date is a working day (Mon-Fri, not a holiday)
func IsWorkingDay(t time.Time) bool {
	wd := t.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	dateStr := t.Format("2006-01-02")
	if nationalHolidays[dateStr] {
		return false
	}
	return true
}

// WorkingDaysSince counts working days between start and now
func WorkingDaysSince(start time.Time) int {
	now := time.Now()
	count := 0
	for d := start; d.Before(now); d = d.AddDate(0, 0, 1) {
		if IsWorkingDay(d) {
			count++
		}
	}
	return count
}

// DaysSince counts calendar days
func DaysSince(t *time.Time) int {
	if t == nil {
		return -1
	}
	return int(time.Since(*t).Hours() / 24)
}
```

---

## Core Workflow Engine Function Signatures (#34, #35)

These are the canonical entry points engineers must implement. Each function is
ctx-first, returns `error`, and is invoked by the cron orchestrator (see
`05-cron-engine.md`). All side effects (WA send, Telegram, DB write) must go
through dedicated services so they are mockable in tests.

```go
package engine

import (
    "context"
    "github.com/google/uuid"
)

// ── Daily entry point per prospect ──────────────────────────
//
// Loaded by cron, fans out to evaluateBlast / evaluateFirstPayment /
// evaluateNurture / evaluateSpecial in priority order. Stops after the
// first action fires (one send per prospect per cron run).
func processProspect(ctx context.Context, prospectID uuid.UUID) error

// ── BD blast sequence (D0–D21 + branching) ──────────────────
//
// Returns nil decision when no D-day is due today. Decision carries
// the resolved trigger_id and template variant so caller can audit.
type BlastDecision struct {
    TriggerID     string                 // e.g. "BD_D7"
    TemplateID    string                 // resolved variant
    Variant       string                 // "high_intent" | "dm_present" | ...
    BranchReasons []string               // for cron_job_logs.resolution_trace
    Skipped       bool                   // true if low-intent shortcut hit
    SkipReason    string
}

func evaluateBlast(
    ctx context.Context,
    p *DataMaster,
    f *ProspectFlags,
) (*BlastDecision, error)

// ── Parallel first-payment chase (D0–D90) ───────────────────
//
// Independent track; runs alongside evaluateBlast for prospects with
// Closing_Status='CLOSING'. Owns the FP_D0/D7/D21/D27 + FP_POST1/4/15
// triggers per matrix in 05-cron-engine.md.
func evaluateFirstPayment(
    ctx context.Context,
    p *DataMaster,
    f *ProspectFlags,
) error

// ── Long-term nurture (D+60 to D+365, repeating) ────────────
//
// For Stage='LEAD' / 'DORMANT' records. Increments nurture_count
// after D+365 and transitions to permanent DORMANT on count >= 3.
func evaluateNurture(
    ctx context.Context,
    p *DataMaster,
    f *ProspectFlags,
) error

// ── Special triggers (WINBACK, seasonal) ────────────────────
//
// One-shot triggers: WINBACK fires X days after lost-deal,
// SEASONAL_NEWYEAR + SEASONAL_LEBARAN fire on calendar date.
func evaluateSpecial(
    ctx context.Context,
    p *DataMaster,
    f *ProspectFlags,
) error

// ── Template resolver (14 branching variables) ──────────────
//
// Pure function — applies priority order from
// 04-api-endpoints.md §2.5 (renewal_imminent → buying_intent →
// legacy branches → default) plus does {{Var}} substitution.
//
// tmplID is the BASE template id (e.g. "TPL-BD-D7"); resolver
// returns the variant id (e.g. "TPL-BD-D7-HIGH-INTENT") and the
// fully-rendered message body.
func resolveTemplate(
    tmplID string,
    p *DataMaster,
    f *ProspectFlags,
) (renderedBody string, err error)

// ── Outbound messaging (HaloAI WA) ──────────────────────────
//
// Wraps HaloAI HTTP API. Implements: 3-attempt exponential backoff
// (1s/3s/9s), per-recipient 24h dedupe (returns nil silently if
// last WA to same phone < 24h), and on permanent failure writes
// action_logs row with status='failed' + alerts BD via alertBD.
type WAMessageMeta struct {
    TriggerID     string
    TemplateID    string
    MasterDataID  uuid.UUID
    Variant       string
}

func sendWAMessage(
    ctx context.Context,
    phone string,
    msg string,
    meta WAMessageMeta,
) error

// ── BD alert via Telegram ───────────────────────────────────
//
// alertType: "send_failed" | "escalation" | "manual_action_due" | "fp_overdue"
// Routing: primary = master_data.Owner_Telegram_ID; fallback to
// TELEGRAM_AE_LEAD_ID if primary unset OR HTTP 4xx response.
func alertBD(
    ctx context.Context,
    p *DataMaster,
    alertType string,
    msg string,
) error

// ── Trigger BD escalation row ───────────────────────────────
//
// Inserts into bd_escalations (see 02-database-schema.md §10).
// Idempotent by (prospect_id, esc_id) WHERE resolved_at IS NULL.
// On insert: fires alertBD with alertType='escalation'.
func triggerEscalation(
    ctx context.Context,
    p *DataMaster,
    escalationID string,  // 'ESC-BD-001' .. 'ESC-BD-009'
) error
```

`ProspectFlags` is the typed snapshot of the 14 branching variables loaded from
`master_data.custom_fields` once per `processProspect` call — see catalog in
`02-database-schema.md` §11.
