// Multi-stage PIC usecase — see context/new/multi-stage-pic-spec.md.
//
// Layered on top of master_data so the dashboard can manage per-stage
// contacts without going through the full client patch flow. Validation is
// thin: workspace + master_id checks come from the route + JWT context;
// the only structural rule we enforce here is "kind must be valid".

package master_data

import (
	"context"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// ContactCreateRequest is the body for POST /clients/{id}/contacts.
type ContactCreateRequest struct {
	Stage      string `json:"stage"`
	Kind       string `json:"kind"`
	Role       string `json:"role"`
	Name       string `json:"name"`
	WA         string `json:"wa"`
	Email      string `json:"email"`
	TelegramID string `json:"telegram_id"`
	IsPrimary  *bool  `json:"is_primary"`
	Notes      string `json:"notes"`
}

// ContactPatchRequest is the body for PATCH /clients/{id}/contacts/{contact_id}.
type ContactPatchRequest struct {
	Stage      *string `json:"stage,omitempty"`
	Kind       *string `json:"kind,omitempty"`
	Role       *string `json:"role,omitempty"`
	Name       *string `json:"name,omitempty"`
	WA         *string `json:"wa,omitempty"`
	Email      *string `json:"email,omitempty"`
	TelegramID *string `json:"telegram_id,omitempty"`
	IsPrimary  *bool   `json:"is_primary,omitempty"`
	Notes      *string `json:"notes,omitempty"`
}

// ListContacts returns all contacts for a client; optional filter by stage/kind.
func (u *usecase) ListContacts(ctx context.Context, workspaceID, masterID string, filter repository.ClientContactFilter) ([]entity.ClientContact, error) {
	if u.contactRepo == nil {
		return nil, apperror.BadRequest("contacts repository not wired")
	}
	if workspaceID == "" || masterID == "" {
		return nil, apperror.ValidationError("workspace_id and master_id required")
	}
	return u.contactRepo.List(ctx, workspaceID, masterID, filter)
}

// CreateContact inserts a new contact for the client. Auto-defaults
// is_primary=true so dashboards can call this with minimal payload.
func (u *usecase) CreateContact(ctx context.Context, workspaceID, masterID string, req ContactCreateRequest) (*entity.ClientContact, error) {
	if u.contactRepo == nil {
		return nil, apperror.BadRequest("contacts repository not wired")
	}
	if workspaceID == "" || masterID == "" {
		return nil, apperror.ValidationError("workspace_id and master_id required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperror.ValidationError("name required")
	}
	if !entity.IsValidContactKind(req.Kind) {
		return nil, apperror.ValidationError("kind must be 'internal' or 'client_side'")
	}
	if strings.TrimSpace(req.Stage) == "" {
		return nil, apperror.ValidationError("stage required")
	}
	primary := true
	if req.IsPrimary != nil {
		primary = *req.IsPrimary
	}
	c := &entity.ClientContact{
		WorkspaceID:  workspaceID,
		MasterDataID: masterID,
		Stage:        req.Stage,
		Kind:         req.Kind,
		Role:         req.Role,
		Name:         strings.TrimSpace(req.Name),
		WA:           req.WA,
		Email:        req.Email,
		TelegramID:   req.TelegramID,
		IsPrimary:    primary,
		Notes:        req.Notes,
	}
	return u.contactRepo.Create(ctx, c)
}

// PatchContact updates fields the caller provides; nils are ignored.
func (u *usecase) PatchContact(ctx context.Context, workspaceID, contactID string, req ContactPatchRequest) (*entity.ClientContact, error) {
	if u.contactRepo == nil {
		return nil, apperror.BadRequest("contacts repository not wired")
	}
	if req.Kind != nil && !entity.IsValidContactKind(*req.Kind) {
		return nil, apperror.ValidationError("kind must be 'internal' or 'client_side'")
	}
	out, err := u.contactRepo.Patch(ctx, workspaceID, contactID, repository.ClientContactPatch{
		Stage:      req.Stage,
		Kind:       req.Kind,
		Role:       req.Role,
		Name:       req.Name,
		WA:         req.WA,
		Email:      req.Email,
		TelegramID: req.TelegramID,
		IsPrimary:  req.IsPrimary,
		Notes:      req.Notes,
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, apperror.NotFound("client_contact", contactID)
	}
	return out, nil
}

// DeleteContact hard-deletes a contact by id.
func (u *usecase) DeleteContact(ctx context.Context, workspaceID, contactID string) error {
	if u.contactRepo == nil {
		return apperror.BadRequest("contacts repository not wired")
	}
	return u.contactRepo.Delete(ctx, workspaceID, contactID)
}
