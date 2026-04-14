// Package custom_field implements per-workspace custom field definition CRUD
// for the Master Data feature.
package custom_field

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// Usecase exposes custom field definition operations.
type Usecase interface {
	List(ctx context.Context, workspaceID string) ([]entity.CustomFieldDefinition, error)
	Get(ctx context.Context, workspaceID, id string) (*entity.CustomFieldDefinition, error)
	Create(ctx context.Context, workspaceID string, req CreateRequest) (*entity.CustomFieldDefinition, error)
	Update(ctx context.Context, workspaceID, id string, req UpdateRequest) (*entity.CustomFieldDefinition, error)
	Delete(ctx context.Context, workspaceID, id string) error
	Reorder(ctx context.Context, workspaceID string, items []repository.ReorderItem) error
}

// CreateRequest is the input for POST /master-data/field-definitions.
type CreateRequest struct {
	FieldKey       string   `json:"field_key"`
	FieldLabel     string   `json:"field_label"`
	FieldType      string   `json:"field_type"`
	IsRequired     bool     `json:"is_required"`
	DefaultValue   string   `json:"default_value"`
	Placeholder    string   `json:"placeholder"`
	Description    string   `json:"description"`
	Options        []string `json:"options"`
	MinValue       *float64 `json:"min_value"`
	MaxValue       *float64 `json:"max_value"`
	RegexPattern   string   `json:"regex_pattern"`
	SortOrder      int      `json:"sort_order"`
	VisibleInTable bool     `json:"visible_in_table"`
	ColumnWidth    int      `json:"column_width"`
}

// UpdateRequest is the input for PUT /master-data/field-definitions/{id}.
// FieldKey is intentionally omitted because keys are immutable post-create.
type UpdateRequest struct {
	FieldLabel     string   `json:"field_label"`
	FieldType      string   `json:"field_type"`
	IsRequired     bool     `json:"is_required"`
	DefaultValue   string   `json:"default_value"`
	Placeholder    string   `json:"placeholder"`
	Description    string   `json:"description"`
	Options        []string `json:"options"`
	MinValue       *float64 `json:"min_value"`
	MaxValue       *float64 `json:"max_value"`
	RegexPattern   string   `json:"regex_pattern"`
	SortOrder      int      `json:"sort_order"`
	VisibleInTable bool     `json:"visible_in_table"`
	ColumnWidth    int      `json:"column_width"`
}

type usecase struct {
	repo repository.CustomFieldDefinitionRepository
}

// New constructs a custom_field usecase.
func New(repo repository.CustomFieldDefinitionRepository) Usecase {
	return &usecase{repo: repo}
}

var fieldKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,49}$`)

func (u *usecase) List(ctx context.Context, workspaceID string) ([]entity.CustomFieldDefinition, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	return u.repo.List(ctx, workspaceID)
}

func (u *usecase) Get(ctx context.Context, workspaceID, id string) (*entity.CustomFieldDefinition, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	out, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, apperror.NotFound("custom_field_definition", "")
	}
	return out, nil
}

func (u *usecase) Create(ctx context.Context, workspaceID string, req CreateRequest) (*entity.CustomFieldDefinition, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if !fieldKeyRe.MatchString(req.FieldKey) {
		return nil, apperror.ValidationError("field_key must be snake_case (lowercase, alphanumeric, underscore)")
	}
	if strings.TrimSpace(req.FieldLabel) == "" {
		return nil, apperror.ValidationError("field_label required")
	}
	if !isValidFieldType(req.FieldType) {
		return nil, apperror.ValidationError("invalid field_type")
	}
	if req.FieldType == entity.FieldTypeSelect && len(req.Options) == 0 {
		return nil, apperror.ValidationError("select field requires at least one option")
	}
	def := &entity.CustomFieldDefinition{
		FieldKey:       req.FieldKey,
		FieldLabel:     req.FieldLabel,
		FieldType:      req.FieldType,
		IsRequired:     req.IsRequired,
		DefaultValue:   req.DefaultValue,
		Placeholder:    req.Placeholder,
		Description:    req.Description,
		MinValue:       req.MinValue,
		MaxValue:       req.MaxValue,
		RegexPattern:   req.RegexPattern,
		SortOrder:      req.SortOrder,
		VisibleInTable: req.VisibleInTable,
		ColumnWidth:    req.ColumnWidth,
	}
	if def.ColumnWidth <= 0 {
		def.ColumnWidth = 120
	}
	if len(req.Options) > 0 {
		raw, err := json.Marshal(req.Options)
		if err != nil {
			return nil, apperror.ValidationError("invalid options")
		}
		def.Options = raw
	}
	out, err := u.repo.Create(ctx, workspaceID, def)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, apperror.Conflict("field_key already exists in this workspace")
		}
		return nil, err
	}
	return out, nil
}

func (u *usecase) Update(ctx context.Context, workspaceID, id string, req UpdateRequest) (*entity.CustomFieldDefinition, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	existing, err := u.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperror.NotFound("custom_field_definition", "")
	}
	if !isValidFieldType(req.FieldType) {
		return nil, apperror.ValidationError("invalid field_type")
	}
	// FieldKey is immutable — preserve from existing.
	updated := *existing
	updated.FieldLabel = req.FieldLabel
	updated.FieldType = req.FieldType
	updated.IsRequired = req.IsRequired
	updated.DefaultValue = req.DefaultValue
	updated.Placeholder = req.Placeholder
	updated.Description = req.Description
	updated.MinValue = req.MinValue
	updated.MaxValue = req.MaxValue
	updated.RegexPattern = req.RegexPattern
	updated.SortOrder = req.SortOrder
	updated.VisibleInTable = req.VisibleInTable
	updated.ColumnWidth = req.ColumnWidth
	if updated.ColumnWidth <= 0 {
		updated.ColumnWidth = 120
	}
	if len(req.Options) > 0 {
		raw, err := json.Marshal(req.Options)
		if err != nil {
			return nil, apperror.ValidationError("invalid options")
		}
		updated.Options = raw
	}
	out, err := u.repo.Update(ctx, workspaceID, id, &updated)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, apperror.NotFound("custom_field_definition", "")
	}
	return out, nil
}

func (u *usecase) Delete(ctx context.Context, workspaceID, id string) error {
	if workspaceID == "" || id == "" {
		return apperror.ValidationError("workspace_id and id required")
	}
	return u.repo.Delete(ctx, workspaceID, id)
}

func (u *usecase) Reorder(ctx context.Context, workspaceID string, items []repository.ReorderItem) error {
	if workspaceID == "" {
		return apperror.ValidationError("workspace_id required")
	}
	if len(items) == 0 {
		return apperror.ValidationError("order required")
	}
	return u.repo.Reorder(ctx, workspaceID, items)
}

func isValidFieldType(s string) bool {
	switch s {
	case entity.FieldTypeText, entity.FieldTypeNumber, entity.FieldTypeDate,
		entity.FieldTypeBoolean, entity.FieldTypeSelect, entity.FieldTypeURL, entity.FieldTypeEmail:
		return true
	}
	return false
}
