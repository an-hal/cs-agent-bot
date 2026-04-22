// Package entity contains domain types for the workflow engine (feat/06).
package entity

import (
	"encoding/json"
	"time"
)

// ─── Workflow status ──────────────────────────────────────────────────────────

// WorkflowStatus is the lifecycle state of a workflow.
type WorkflowStatus string

const (
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusDraft    WorkflowStatus = "draft"
	WorkflowStatusDisabled WorkflowStatus = "disabled"
)

// ─── Workflow ─────────────────────────────────────────────────────────────────

// Workflow is a top-level pipeline definition (workspace-scoped).
type Workflow struct {
	ID          string         `db:"id"           json:"id"`
	WorkspaceID string         `db:"workspace_id" json:"workspace_id"`
	Name        string         `db:"name"         json:"name"`
	Icon        *string        `db:"icon"         json:"icon"`
	Slug        string         `db:"slug"         json:"slug"`
	Description *string        `db:"description"  json:"description"`
	Status      WorkflowStatus `db:"status"       json:"status"`
	StageFilter []string       `db:"stage_filter" json:"stage_filter"`
	CreatedBy   *string        `db:"created_by"   json:"created_by"`
	UpdatedBy   *string        `db:"updated_by"   json:"updated_by"`
	CreatedAt   time.Time      `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"   json:"updated_at"`
}

// WorkflowListItem is a lightweight workflow for list responses (includes counts).
type WorkflowListItem struct {
	Workflow
	NodeCount int `json:"node_count"`
	EdgeCount int `json:"edge_count"`
}

// WorkflowFull includes all nested canvas and pipeline config entities.
type WorkflowFull struct {
	Workflow
	Nodes   []WorkflowNode   `json:"nodes"`
	Edges   []WorkflowEdge   `json:"edges"`
	Steps   []WorkflowStep   `json:"steps"`
	Tabs    []PipelineTab    `json:"tabs"`
	Stats   []PipelineStat   `json:"stats"`
	Columns []PipelineColumn `json:"columns"`
}

// ─── WorkflowNode ─────────────────────────────────────────────────────────────

// WorkflowNode is a React Flow node stored in workflow_nodes.
type WorkflowNode struct {
	ID         string    `db:"id"          json:"id"`
	WorkflowID string    `db:"workflow_id" json:"workflow_id"`
	NodeID     string    `db:"node_id"     json:"node_id"`
	NodeType   string    `db:"node_type"   json:"node_type"`
	PositionX  float64   `db:"position_x"  json:"position_x"`
	PositionY  float64   `db:"position_y"  json:"position_y"`
	Width      *float64  `db:"width"       json:"width,omitempty"`
	Height     *float64  `db:"height"      json:"height,omitempty"`
	Data       json.RawMessage `db:"data"  json:"data" swaggertype:"object"`
	Draggable  bool      `db:"draggable"   json:"draggable"`
	Selectable bool      `db:"selectable"  json:"selectable"`
	Connectable bool     `db:"connectable" json:"connectable"`
	ZIndex     int       `db:"z_index"     json:"z_index"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"  json:"updated_at"`
}

// NodeData is the typed representation of WorkflowNode.Data (JSONB).
type NodeData struct {
	Category    string `json:"category,omitempty"`
	Label       string `json:"label,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
	TemplateID  string `json:"templateId,omitempty"`
	TriggerID   string `json:"triggerId,omitempty"`
	Timing      string `json:"timing,omitempty"`
	Condition   string `json:"condition,omitempty"`
	StopIf      string `json:"stopIf,omitempty"`
	SentFlag    string `json:"sentFlag,omitempty"`
	// Zone-only fields
	Color string `json:"color,omitempty"`
	Bg    string `json:"bg,omitempty"`
}

// GetNodeData parses the JSONB Data field into a typed struct.
func (n *WorkflowNode) GetNodeData() (*NodeData, error) {
	var d NodeData
	if err := json.Unmarshal(n.Data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// ─── WorkflowEdge ─────────────────────────────────────────────────────────────

// WorkflowEdge is a React Flow edge stored in workflow_edges.
type WorkflowEdge struct {
	ID           string          `db:"id"             json:"id"`
	WorkflowID   string          `db:"workflow_id"    json:"workflow_id"`
	EdgeID       string          `db:"edge_id"        json:"edge_id"`
	SourceNodeID string          `db:"source_node_id" json:"source"`
	TargetNodeID string          `db:"target_node_id" json:"target"`
	SourceHandle *string         `db:"source_handle"  json:"sourceHandle,omitempty"`
	TargetHandle *string         `db:"target_handle"  json:"targetHandle,omitempty"`
	Label        *string         `db:"label"          json:"label,omitempty"`
	Animated     bool            `db:"animated"       json:"animated"`
	Style        json.RawMessage `db:"style"          json:"style,omitempty" swaggertype:"object"`
	CreatedAt    time.Time       `db:"created_at"     json:"created_at"`
}

// ─── WorkflowStep ─────────────────────────────────────────────────────────────

// WorkflowStep is a pipeline step configuration record in workflow_steps.
type WorkflowStep struct {
	ID         string    `db:"id"          json:"id"`
	WorkflowID string    `db:"workflow_id" json:"workflow_id"`
	StepKey    string    `db:"step_key"    json:"key"`
	Label      string    `db:"label"       json:"label"`
	Phase      string    `db:"phase"       json:"phase"`
	Icon       *string   `db:"icon"        json:"icon"`
	Description *string  `db:"description" json:"description"`
	SortOrder  int       `db:"sort_order"  json:"sort_order"`

	Timing    *string `db:"timing"    json:"timing"`
	Condition *string `db:"condition" json:"condition"`
	StopIf    *string `db:"stop_if"   json:"stopIf"`
	SentFlag  *string `db:"sent_flag" json:"sentFlag"`
	TemplateID *string `db:"template_id" json:"templateId"`

	MessageTemplateID *string `db:"message_template_id" json:"messageTemplateId"`
	EmailTemplateID   *string `db:"email_template_id"   json:"emailTemplateId"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ─── Pipeline config ──────────────────────────────────────────────────────────

// PipelineTab is a filter tab configuration for a workflow pipeline view.
type PipelineTab struct {
	ID         string  `db:"id"          json:"id"`
	WorkflowID string  `db:"workflow_id" json:"workflow_id"`
	TabKey     string  `db:"tab_key"     json:"key"`
	Label      string  `db:"label"       json:"label"`
	Icon       *string `db:"icon"        json:"icon"`
	Filter     string  `db:"filter"      json:"filter"`
	SortOrder  int     `db:"sort_order"  json:"sort_order"`
}

// PipelineStat is a stat card configuration for a workflow pipeline view.
type PipelineStat struct {
	ID         string  `db:"id"          json:"id"`
	WorkflowID string  `db:"workflow_id" json:"workflow_id"`
	StatKey    string  `db:"stat_key"    json:"key"`
	Label      string  `db:"label"       json:"label"`
	Metric     string  `db:"metric"      json:"metric"`
	Color      *string `db:"color"       json:"color"`
	Border     *string `db:"border"      json:"border"`
	SortOrder  int     `db:"sort_order"  json:"sort_order"`
}

// PipelineColumn is a table column configuration for a workflow pipeline view.
type PipelineColumn struct {
	ID         string `db:"id"          json:"id"`
	WorkflowID string `db:"workflow_id" json:"workflow_id"`
	ColumnKey  string `db:"column_key"  json:"key"`
	Field      string `db:"field"       json:"field"`
	Label      string `db:"label"       json:"label"`
	Width      int    `db:"width"       json:"width"`
	Visible    bool   `db:"visible"     json:"visible"`
	SortOrder  int    `db:"sort_order"  json:"sort_order"`
}
