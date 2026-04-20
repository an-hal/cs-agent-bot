package collection

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

// ApplyCollectionSchemaChange is the dispatcher that executes a pending
// `collection_schema_change` approval. Follows the shape of
// invoice.ApplyMarkPaid so a future central approval endpoint can call this
// without any per-feature knowledge.
//
// The dispatcher handles 4 ops:
//   - create_collection  : insert collection + fields
//   - delete_collection  : soft-delete collection (cascade handles children)
//   - add_field          : insert one field
//   - delete_field       : delete field + strip key from records
//
// All paths mark the approval as approved on success and write an audit log entry.
func (u *usecase) ApplyCollectionSchemaChange(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.ApprovalRequest, error) {
	if workspaceID == "" || approvalID == "" {
		return nil, apperror.ValidationError("workspace_id and approval_id required")
	}
	ar, err := u.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return nil, err
	}
	if ar == nil {
		return nil, apperror.NotFound("approval_request", approvalID)
	}
	if ar.RequestType != entity.CollectionSchemaChangeType {
		return nil, apperror.BadRequest("approval is not a collection_schema_change request")
	}
	if ar.Status != entity.ApprovalStatusPending {
		return nil, apperror.BadRequest("approval is not pending (status=" + ar.Status + ")")
	}
	if ar.MakerEmail == checkerEmail {
		return nil, apperror.BadRequest("cannot approve your own request")
	}

	op, _ := ar.Payload["op"].(string)
	switch op {
	case entity.OpCreateCollection:
		if err := u.applyCreateCollection(ctx, workspaceID, ar); err != nil {
			return nil, err
		}
	case entity.OpDeleteCollection:
		if err := u.applyDeleteCollection(ctx, workspaceID, ar); err != nil {
			return nil, err
		}
	case entity.OpAddField:
		if err := u.applyAddField(ctx, workspaceID, ar); err != nil {
			return nil, err
		}
	case entity.OpDeleteField:
		if err := u.applyDeleteField(ctx, workspaceID, ar); err != nil {
			return nil, err
		}
	default:
		return nil, apperror.BadRequest("unsupported op in approval payload: " + op)
	}

	if err := u.approvalRepo.UpdateStatus(ctx, workspaceID, ar.ID, entity.ApprovalStatusApproved, checkerEmail, ""); err != nil {
		return nil, fmt.Errorf("mark approval approved: %w", err)
	}
	ar.Status = entity.ApprovalStatusApproved
	ar.CheckerEmail = checkerEmail

	u.audit(ctx, workspaceID, checkerEmail, "collection.approval_applied", ar.ID, ar.Description, op)
	return ar, nil
}

func (u *usecase) applyCreateCollection(ctx context.Context, workspaceID string, ar *entity.ApprovalRequest) error {
	slug, _ := ar.Payload["slug"].(string)
	name, _ := ar.Payload["name"].(string)
	desc, _ := ar.Payload["desc"].(string)
	icon, _ := ar.Payload["icon"].(string)
	actor, _ := ar.Payload["actor"].(string)
	perms, _ := ar.Payload["permissions"].(map[string]any)

	// Re-check slug uniqueness at apply time — another approval could have
	// won the race between enqueue and approve.
	existing, err := u.collectionRepo.GetBySlug(ctx, workspaceID, slug)
	if err != nil {
		return err
	}
	if existing != nil {
		return apperror.Conflict("slug already in use: " + slug)
	}

	c, err := u.collectionRepo.Create(ctx, &entity.Collection{
		WorkspaceID: workspaceID,
		Slug:        slug,
		Name:        name,
		Description: desc,
		Icon:        icon,
		Permissions: coalesceMap(perms),
		CreatedBy:   actor,
	})
	if err != nil {
		return err
	}

	fields, err := decodeFieldInputs(ar.Payload["fields"])
	if err != nil {
		return err
	}
	for _, f := range fields {
		_, err := u.fieldRepo.Create(ctx, &entity.CollectionField{
			CollectionID: c.ID,
			Key:          f.Key,
			Label:        f.Label,
			Type:         f.Type,
			Required:     f.Required,
			Options:      coalesceMap(f.Options),
			DefaultValue: f.DefaultValue,
			Order:        f.Order,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (u *usecase) applyDeleteCollection(ctx context.Context, workspaceID string, ar *entity.ApprovalRequest) error {
	id, _ := ar.Payload["collection_id"].(string)
	if id == "" {
		return apperror.BadRequest("collection_id missing in payload")
	}
	return u.collectionRepo.SoftDelete(ctx, workspaceID, id)
}

func (u *usecase) applyAddField(ctx context.Context, workspaceID string, ar *entity.ApprovalRequest) error {
	id, _ := ar.Payload["collection_id"].(string)
	c, err := u.collectionRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if c == nil {
		return apperror.NotFound("collection", id)
	}
	fields, err := decodeFieldInputs([]any{ar.Payload["field"]})
	if err != nil {
		return err
	}
	if len(fields) != 1 {
		return apperror.BadRequest("field missing in payload")
	}
	f := fields[0]
	_, err = u.fieldRepo.Create(ctx, &entity.CollectionField{
		CollectionID: c.ID,
		Key:          f.Key,
		Label:        f.Label,
		Type:         f.Type,
		Required:     f.Required,
		Options:      coalesceMap(f.Options),
		DefaultValue: f.DefaultValue,
		Order:        f.Order,
	})
	return err
}

func (u *usecase) applyDeleteField(ctx context.Context, workspaceID string, ar *entity.ApprovalRequest) error {
	id, _ := ar.Payload["collection_id"].(string)
	fieldID, _ := ar.Payload["field_id"].(string)
	fieldKey, _ := ar.Payload["field_key"].(string)
	c, err := u.collectionRepo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if c == nil {
		return apperror.NotFound("collection", id)
	}
	if err := u.fieldRepo.Delete(ctx, c.ID, fieldID); err != nil {
		return err
	}
	return u.fieldRepo.StripKeyFromRecords(ctx, c.ID, fieldKey)
}

// decodeFieldInputs round-trips `[]FieldInput` through JSON. Approval payloads
// come back from the repo as `[]any` of `map[string]any`, so a direct type
// assertion isn't enough.
func decodeFieldInputs(v any) ([]FieldInput, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("re-marshal field list: %w", err)
	}
	var out []FieldInput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode field inputs: %w", err)
	}
	return out, nil
}
