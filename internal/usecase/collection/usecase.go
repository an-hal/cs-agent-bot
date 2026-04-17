// Package collection implements user-defined generic tables (feat/10).
// Schema changes route through the checker-maker approval queue; record CRUD is
// direct but audit-logged. Strict per-workspace scoping on every call.
package collection

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

// Usecase is the public surface for the Collections feature.
type Usecase interface {
	// Collection meta
	ListCollections(ctx context.Context, workspaceID string) ([]entity.Collection, error)
	GetCollection(ctx context.Context, workspaceID, id string) (*entity.Collection, error)

	// Schema mutations — return approval requests (202 semantics).
	RequestCreateCollection(ctx context.Context, workspaceID, actor string, req CreateCollectionRequest) (*entity.ApprovalRequest, error)
	RequestDeleteCollection(ctx context.Context, workspaceID, actor, id string) (*entity.ApprovalRequest, error)
	RequestAddField(ctx context.Context, workspaceID, actor, collectionID string, req FieldInput) (*entity.ApprovalRequest, error)
	RequestDeleteField(ctx context.Context, workspaceID, actor, collectionID, fieldID string) (*entity.ApprovalRequest, error)

	// Direct schema ops (no approval — just meta edits / field reordering).
	UpdateCollectionMeta(ctx context.Context, workspaceID, actor, id string, req UpdateCollectionMetaRequest) (*entity.Collection, error)
	UpdateFieldMeta(ctx context.Context, workspaceID, actor, collectionID, fieldID string, req UpdateFieldMetaRequest) (*entity.CollectionField, error)

	// Apply dispatcher — executes a pending collection_schema_change approval.
	ApplyCollectionSchemaChange(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.ApprovalRequest, error)

	// Record CRUD
	ListRecords(ctx context.Context, workspaceID, collectionID string, req ListRecordsRequest) ([]entity.CollectionRecord, int, error)
	DistinctValues(ctx context.Context, workspaceID, collectionID, fieldKey string, limit int, filter string) (*entity.DistinctResult, error)
	CreateRecord(ctx context.Context, workspaceID, actor, collectionID string, data map[string]any) (*entity.CollectionRecord, error)
	UpdateRecord(ctx context.Context, workspaceID, actor, collectionID, recordID string, data map[string]any) (*entity.CollectionRecord, error)
	DeleteRecord(ctx context.Context, workspaceID, actor, collectionID, recordID string) error
	BulkRecords(ctx context.Context, workspaceID, actor, collectionID string, req BulkRecordsRequest) (*BulkRecordsResult, error)
}

// CreateCollectionRequest is the input for POST /collections (maker side).
type CreateCollectionRequest struct {
	Slug        string         `json:"slug"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Fields      []FieldInput   `json:"fields"`
	Permissions map[string]any `json:"permissions"`
}

// FieldInput is the serialized form of a field for create / add-field requests.
type FieldInput struct {
	Key          string         `json:"key"`
	Label        string         `json:"label"`
	Type         string         `json:"type"`
	Required     bool           `json:"required"`
	Options      map[string]any `json:"options"`
	DefaultValue any            `json:"default_value"`
	Order        int            `json:"order"`
}

// UpdateCollectionMetaRequest updates display meta (no schema change).
type UpdateCollectionMetaRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Permissions map[string]any `json:"permissions"`
}

// UpdateFieldMetaRequest permits label/required/order/options edits.
// Type changes are not supported — delete + recreate instead.
type UpdateFieldMetaRequest struct {
	Label    string         `json:"label"`
	Required bool           `json:"required"`
	Order    int            `json:"order"`
	Options  map[string]any `json:"options"`
}

// ListRecordsRequest matches spec §GET /collections/{id}/records.
type ListRecordsRequest struct {
	Limit  int
	Offset int
	Sort   string // e.g. "created_at:desc", "data.title:asc"
	Filter string // filter DSL for JSONB
	Search string
}

// BulkRecordsRequest is the payload for POST /records/bulk.
type BulkRecordsRequest struct {
	Op   string         `json:"op"` // delete | update
	IDs  []string       `json:"ids"`
	Data map[string]any `json:"data"` // for update
}

// BulkRecordsResult reports how many rows were affected.
type BulkRecordsResult struct {
	Affected int `json:"affected"`
}

var (
	slugRe     = regexp.MustCompile(`^[a-z0-9-]{3,64}$`)
	fieldKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

type usecase struct {
	collectionRepo repository.CollectionRepository
	fieldRepo      repository.CollectionFieldRepository
	recordRepo     repository.CollectionRecordRepository
	approvalRepo   repository.ApprovalRequestRepository
	logRepo        repository.LogRepository
	masterDataRepo repository.MasterDataRepository // for link_client FK validation
	tracer         tracer.Tracer
	logger         zerolog.Logger
}

// New constructs the Collections usecase.
func New(
	collectionRepo repository.CollectionRepository,
	fieldRepo repository.CollectionFieldRepository,
	recordRepo repository.CollectionRecordRepository,
	approvalRepo repository.ApprovalRequestRepository,
	logRepo repository.LogRepository,
	masterDataRepo repository.MasterDataRepository,
	tr tracer.Tracer,
	logger zerolog.Logger,
) Usecase {
	return &usecase{
		collectionRepo: collectionRepo,
		fieldRepo:      fieldRepo,
		recordRepo:     recordRepo,
		approvalRepo:   approvalRepo,
		logRepo:        logRepo,
		masterDataRepo: masterDataRepo,
		tracer:         tr,
		logger:         logger,
	}
}

// resolveCollection loads and workspace-scopes a collection by ID.
func (u *usecase) resolveCollection(ctx context.Context, workspaceID, id string) (*entity.Collection, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and collection id required")
	}
	c, err := u.collectionRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, apperror.NotFound("collection", id)
	}
	return c, nil
}

// loadFields fetches the schema fields for a collection.
func (u *usecase) loadFields(ctx context.Context, collectionID string) ([]entity.CollectionField, error) {
	return u.fieldRepo.ListByCollection(ctx, collectionID)
}

// ListCollections returns all active collections in the workspace.
func (u *usecase) ListCollections(ctx context.Context, workspaceID string) ([]entity.Collection, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	return u.collectionRepo.List(ctx, workspaceID)
}

// GetCollection returns one collection with its field schema attached.
func (u *usecase) GetCollection(ctx context.Context, workspaceID, id string) (*entity.Collection, error) {
	c, err := u.resolveCollection(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	fields, err := u.loadFields(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	c.Fields = fields
	c.FieldCount = len(fields)
	return c, nil
}

// validateFieldInput checks one FieldInput for well-formedness.
func validateFieldInput(f FieldInput) error {
	if !fieldKeyRe.MatchString(f.Key) {
		return apperror.ValidationError("field key must match ^[a-z][a-z0-9_]{0,63}$: " + f.Key)
	}
	if strings.TrimSpace(f.Label) == "" {
		return apperror.ValidationError("field label required: " + f.Key)
	}
	if !entity.IsValidColFieldType(f.Type) {
		return apperror.ValidationError("invalid field type: " + f.Type)
	}
	if f.Type == entity.ColFieldEnum || f.Type == entity.ColFieldMultiEnum {
		if _, ok := f.Options["choices"]; !ok {
			return apperror.ValidationError("enum/multi_enum fields require options.choices: " + f.Key)
		}
	}
	return nil
}

// RequestCreateCollection validates input and enqueues an approval request.
func (u *usecase) RequestCreateCollection(ctx context.Context, workspaceID, actor string, req CreateCollectionRequest) (*entity.ApprovalRequest, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if !slugRe.MatchString(req.Slug) {
		return nil, apperror.ValidationError("slug must match ^[a-z0-9-]{3,64}$")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperror.ValidationError("name required")
	}
	if len(req.Fields) == 0 {
		return nil, apperror.ValidationError("at least one field is required")
	}
	if len(req.Fields) > entity.MaxFieldsPerCollection {
		return nil, apperror.ValidationError("too many fields — max is 30")
	}
	seen := map[string]bool{}
	for _, f := range req.Fields {
		if err := validateFieldInput(f); err != nil {
			return nil, err
		}
		if seen[f.Key] {
			return nil, apperror.ValidationError("duplicate field key: " + f.Key)
		}
		seen[f.Key] = true
	}

	// Hard limit — 50 active collections / workspace.
	n, err := u.collectionRepo.CountActiveByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if n >= entity.MaxCollectionsPerWorkspace {
		return nil, apperror.ValidationError("workspace has reached max collections (50)")
	}

	// Slug uniqueness check (active).
	existing, err := u.collectionRepo.GetBySlug(ctx, workspaceID, req.Slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperror.ValidationError("slug already in use: " + req.Slug)
	}

	payload := map[string]any{
		"op":   entity.OpCreateCollection,
		"slug": req.Slug,
		"name": req.Name,
		"desc": req.Description,
		"icon": req.Icon,
		"fields":      req.Fields,
		"permissions": coalesceMap(req.Permissions),
		"actor":       actor,
	}
	desc := "Create collection: " + req.Name + " (" + req.Slug + ")"
	return u.enqueueApproval(ctx, workspaceID, actor, desc, payload)
}

// RequestDeleteCollection enqueues a soft-delete approval.
func (u *usecase) RequestDeleteCollection(ctx context.Context, workspaceID, actor, id string) (*entity.ApprovalRequest, error) {
	c, err := u.resolveCollection(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"op":            entity.OpDeleteCollection,
		"collection_id": c.ID,
		"name":          c.Name,
		"actor":         actor,
	}
	desc := "Delete collection: " + c.Name + " (" + c.Slug + ")"
	return u.enqueueApproval(ctx, workspaceID, actor, desc, payload)
}

// RequestAddField enqueues an add-field approval after validation + hard-limit checks.
func (u *usecase) RequestAddField(ctx context.Context, workspaceID, actor, collectionID string, req FieldInput) (*entity.ApprovalRequest, error) {
	c, err := u.resolveCollection(ctx, workspaceID, collectionID)
	if err != nil {
		return nil, err
	}
	if err := validateFieldInput(req); err != nil {
		return nil, err
	}
	count, err := u.fieldRepo.CountByCollection(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	if count >= entity.MaxFieldsPerCollection {
		return nil, apperror.ValidationError("collection has reached max fields (30)")
	}
	// Duplicate key guard.
	existing, err := u.fieldRepo.GetByKey(ctx, c.ID, req.Key)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, apperror.ValidationError("field key already exists: " + req.Key)
	}

	payload := map[string]any{
		"op":            entity.OpAddField,
		"collection_id": c.ID,
		"field":         req,
		"actor":         actor,
	}
	desc := "Add field '" + req.Label + "' to " + c.Name
	return u.enqueueApproval(ctx, workspaceID, actor, desc, payload)
}

// RequestDeleteField enqueues a delete-field approval.
func (u *usecase) RequestDeleteField(ctx context.Context, workspaceID, actor, collectionID, fieldID string) (*entity.ApprovalRequest, error) {
	c, err := u.resolveCollection(ctx, workspaceID, collectionID)
	if err != nil {
		return nil, err
	}
	f, err := u.fieldRepo.GetByID(ctx, c.ID, fieldID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, apperror.NotFound("collection_field", fieldID)
	}
	payload := map[string]any{
		"op":            entity.OpDeleteField,
		"collection_id": c.ID,
		"field_id":      f.ID,
		"field_key":     f.Key,
		"actor":         actor,
	}
	desc := "Remove field '" + f.Label + "' from " + c.Name
	return u.enqueueApproval(ctx, workspaceID, actor, desc, payload)
}

// enqueueApproval writes a pending approval_requests row (72h TTL).
func (u *usecase) enqueueApproval(ctx context.Context, workspaceID, actor, desc string, payload map[string]any) (*entity.ApprovalRequest, error) {
	now := time.Now().UTC()
	ar := &entity.ApprovalRequest{
		WorkspaceID: workspaceID,
		RequestType: entity.CollectionSchemaChangeType,
		Description: desc,
		Payload:     payload,
		Status:      entity.ApprovalStatusPending,
		MakerEmail:  actor,
		MakerAt:     now,
		ExpiresAt:   now.Add(72 * time.Hour),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return u.approvalRepo.Create(ctx, ar)
}

// UpdateCollectionMeta applies a direct (non-approval) meta edit.
func (u *usecase) UpdateCollectionMeta(ctx context.Context, workspaceID, actor, id string, req UpdateCollectionMetaRequest) (*entity.Collection, error) {
	c, err := u.resolveCollection(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = c.Name
	}
	out, err := u.collectionRepo.UpdateMeta(ctx, workspaceID, c.ID, req.Name, req.Description, req.Icon, coalesceMap(req.Permissions))
	if err != nil {
		return nil, err
	}
	u.audit(ctx, workspaceID, actor, "collection.update_meta", c.ID, c.Name, "")
	return out, nil
}

// UpdateFieldMeta applies a direct (non-approval) meta/reorder edit to a field.
func (u *usecase) UpdateFieldMeta(ctx context.Context, workspaceID, actor, collectionID, fieldID string, req UpdateFieldMetaRequest) (*entity.CollectionField, error) {
	c, err := u.resolveCollection(ctx, workspaceID, collectionID)
	if err != nil {
		return nil, err
	}
	f, err := u.fieldRepo.GetByID(ctx, c.ID, fieldID)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, apperror.NotFound("collection_field", fieldID)
	}
	label := req.Label
	if strings.TrimSpace(label) == "" {
		label = f.Label
	}
	out, err := u.fieldRepo.UpdateMeta(ctx, c.ID, f.ID, label, req.Required, req.Order, coalesceMap(req.Options))
	if err != nil {
		return nil, err
	}
	u.audit(ctx, workspaceID, actor, "collection.update_field_meta", c.ID, c.Name, f.Key)
	return out, nil
}

// audit writes an activity_log entry for a collection action.
func (u *usecase) audit(ctx context.Context, workspaceID, actor, action, refID, target, detail string) {
	if u.logRepo == nil {
		return
	}
	entry := entity.ActivityLog{
		WorkspaceID:  workspaceID,
		Category:     "data",
		ActorType:    "human",
		Actor:        actor,
		ActorEmail:   actor,
		Action:       action,
		Target:       target,
		Detail:       detail,
		RefID:        refID,
		ResourceType: "collection",
		OccurredAt:   time.Now().UTC(),
	}
	if err := u.logRepo.AppendActivity(ctx, entry); err != nil {
		u.logger.Warn().Err(err).Str("action", action).Msg("collection: audit append failed")
	}
}

// coalesceMap returns an empty map when m is nil so JSON marshalling stays stable.
func coalesceMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

