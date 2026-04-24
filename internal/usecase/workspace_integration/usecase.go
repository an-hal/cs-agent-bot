// Package workspaceintegration stores per-workspace third-party credentials
// (HaloAI, Telegram, Paper.id, SMTP). Secret fields are redacted on read.
// Encryption-at-rest is deferred; clients must NEVER log the raw config.
package workspaceintegration

import (
	"context"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// RedactedMarker is placed in place of secret-ish config values on read.
const RedactedMarker = "***REDACTED***"

// Keys whose VALUES are always redacted on read. Match is case-insensitive
// and substring-based so nested shapes like `smtp_password` and `api_key` both hit.
var secretKeyMarkers = []string{"token", "secret", "password", "api_key", "key"}

type Usecase interface {
	Get(ctx context.Context, workspaceID, provider string) (*entity.WorkspaceIntegration, error)
	List(ctx context.Context, workspaceID string) ([]entity.WorkspaceIntegration, error)
	Upsert(ctx context.Context, req UpsertRequest) (*entity.WorkspaceIntegration, error)
	Delete(ctx context.Context, workspaceID, provider string) error
	// RequestKeyChange enqueues a checker-maker approval for an integration
	// config update. Use when the caller wants 4-eyes on credential rotation.
	RequestKeyChange(ctx context.Context, req UpsertRequest) (*entity.ApprovalRequest, error)
	// ApplyApprovedKeyChange executes a pending integration_key_change approval.
	ApplyApprovedKeyChange(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.WorkspaceIntegration, error)
}

type UpsertRequest struct {
	WorkspaceID string         `json:"workspace_id"`
	Provider    string         `json:"provider"`
	DisplayName string         `json:"display_name"`
	Config      map[string]any `json:"config"`
	IsActive    *bool          `json:"is_active,omitempty"`
	ActorEmail  string         `json:"-"`
}

type usecase struct {
	repo         repository.WorkspaceIntegrationRepository
	approvalRepo repository.ApprovalRequestRepository
}

func New(repo repository.WorkspaceIntegrationRepository) Usecase {
	return &usecase{repo: repo}
}

// NewWithApproval attaches the approval_requests repo so RequestKeyChange /
// ApplyApprovedKeyChange work. Without it those methods return a configuration
// error (approval gate disabled).
func NewWithApproval(repo repository.WorkspaceIntegrationRepository, approvalRepo repository.ApprovalRequestRepository) Usecase {
	return &usecase{repo: repo, approvalRepo: approvalRepo}
}

func validProvider(p string) bool {
	switch p {
	case entity.IntegrationProviderHaloAI,
		entity.IntegrationProviderTelegram,
		entity.IntegrationProviderPaperID,
		entity.IntegrationProviderSMTP:
		return true
	}
	return false
}

func (u *usecase) Get(ctx context.Context, workspaceID, provider string) (*entity.WorkspaceIntegration, error) {
	if workspaceID == "" || provider == "" {
		return nil, apperror.ValidationError("workspace_id and provider required")
	}
	w, err := u.repo.GetByProvider(ctx, workspaceID, provider)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, apperror.NotFound("integration", "integration not found for provider "+provider)
	}
	redactConfig(w)
	return w, nil
}

func (u *usecase) List(ctx context.Context, workspaceID string) ([]entity.WorkspaceIntegration, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	items, err := u.repo.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		redactConfig(&items[i])
	}
	return items, nil
}

func (u *usecase) Upsert(ctx context.Context, req UpsertRequest) (*entity.WorkspaceIntegration, error) {
	if req.WorkspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if !validProvider(req.Provider) {
		return nil, apperror.ValidationError("unsupported provider: " + req.Provider)
	}
	if req.Config == nil {
		req.Config = map[string]any{}
	}
	// Reject redacted markers from the client — they indicate FE forgot to drop the field.
	if containsRedacted(req.Config) {
		return nil, apperror.ValidationError("config contains redacted placeholder; send the real secret or omit the field")
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	out, err := u.repo.Upsert(ctx, &entity.WorkspaceIntegration{
		WorkspaceID: req.WorkspaceID,
		Provider:    req.Provider,
		DisplayName: strings.TrimSpace(req.DisplayName),
		Config:      req.Config,
		IsActive:    active,
		UpdatedBy:   strings.ToLower(req.ActorEmail),
	})
	if err != nil {
		return nil, err
	}
	redactConfig(out)
	return out, nil
}

func (u *usecase) Delete(ctx context.Context, workspaceID, provider string) error {
	if workspaceID == "" || provider == "" {
		return apperror.ValidationError("workspace_id and provider required")
	}
	return u.repo.Delete(ctx, workspaceID, provider)
}

// redactConfig replaces secret-ish values in place. Non-string values are
// replaced with the marker as well, to avoid leaking partial data.
func redactConfig(w *entity.WorkspaceIntegration) {
	if w == nil {
		return
	}
	for k := range w.Config {
		if isSecretKey(k) {
			w.Config[k] = RedactedMarker
		}
	}
}

func isSecretKey(k string) bool {
	lk := strings.ToLower(k)
	for _, m := range secretKeyMarkers {
		if strings.Contains(lk, m) {
			return true
		}
	}
	return false
}

func containsRedacted(cfg map[string]any) bool {
	for _, v := range cfg {
		if s, ok := v.(string); ok && s == RedactedMarker {
			return true
		}
	}
	return false
}

// RequestKeyChange enqueues an approval for rotating the integration config.
// The full new config is stored in the approval payload; apply step writes
// it to the live row. Caller must be non-nil for the approvalRepo to work.
func (u *usecase) RequestKeyChange(ctx context.Context, req UpsertRequest) (*entity.ApprovalRequest, error) {
	if u.approvalRepo == nil {
		return nil, apperror.InternalErrorWithMessage("approval repo not wired for integration_key_change", nil)
	}
	if req.WorkspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if !validProvider(req.Provider) {
		return nil, apperror.ValidationError("unsupported provider: " + req.Provider)
	}
	if req.Config == nil {
		req.Config = map[string]any{}
	}
	if containsRedacted(req.Config) {
		return nil, apperror.ValidationError("config contains redacted placeholder; send the real secret or omit the field")
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	desc := "Rotate " + req.Provider + " integration credentials"
	return u.approvalRepo.Create(ctx, &entity.ApprovalRequest{
		WorkspaceID: req.WorkspaceID,
		RequestType: entity.ApprovalTypeIntegrationKeyChange,
		Description: desc,
		Payload: map[string]any{
			"provider":     req.Provider,
			"display_name": req.DisplayName,
			"config":       req.Config,
			"is_active":    active,
		},
		MakerEmail: strings.ToLower(req.ActorEmail),
	})
}

// ApplyApprovedKeyChange writes the approved config to the integration row.
func (u *usecase) ApplyApprovedKeyChange(ctx context.Context, workspaceID, approvalID, checkerEmail string) (*entity.WorkspaceIntegration, error) {
	if u.approvalRepo == nil {
		return nil, apperror.InternalErrorWithMessage("approval repo not wired", nil)
	}
	ar, err := u.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return nil, err
	}
	if ar == nil || ar.RequestType != entity.ApprovalTypeIntegrationKeyChange {
		return nil, apperror.NotFound("integration_key_change approval", approvalID)
	}
	if ar.Status != entity.ApprovalStatusPending {
		return nil, apperror.BadRequest("approval not pending (status=" + ar.Status + ")")
	}
	if ar.MakerEmail == strings.ToLower(checkerEmail) {
		return nil, apperror.BadRequest("cannot approve your own request")
	}
	provider, _ := ar.Payload["provider"].(string)
	displayName, _ := ar.Payload["display_name"].(string)
	cfg, _ := ar.Payload["config"].(map[string]any)
	isActive, _ := ar.Payload["is_active"].(bool)

	out, err := u.repo.Upsert(ctx, &entity.WorkspaceIntegration{
		WorkspaceID: workspaceID,
		Provider:    provider,
		DisplayName: displayName,
		Config:      cfg,
		IsActive:    isActive,
		UpdatedBy:   strings.ToLower(checkerEmail),
	})
	if err != nil {
		return nil, err
	}
	if err := u.approvalRepo.UpdateStatus(ctx, workspaceID, approvalID, entity.ApprovalStatusApproved, checkerEmail, ""); err != nil {
		return nil, err
	}
	redactConfig(out)
	return out, nil
}
