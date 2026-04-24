// Package userpreferences exposes per-user, per-workspace preference storage.
// The stored value is free-form JSON owned by the FE (theme, column visibility,
// sidebar state, feed interval, etc.). BE treats it as an opaque blob.
package userpreferences

import (
	"context"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

type Usecase interface {
	Get(ctx context.Context, workspaceID, userEmail, namespace string) (*entity.UserPreference, error)
	List(ctx context.Context, workspaceID, userEmail string) ([]entity.UserPreference, error)
	Upsert(ctx context.Context, req UpsertRequest) (*entity.UserPreference, error)
	Delete(ctx context.Context, workspaceID, userEmail, namespace string) error
}

type UpsertRequest struct {
	WorkspaceID string         `json:"workspace_id"`
	UserEmail   string         `json:"user_email"`
	Namespace   string         `json:"namespace"`
	Value       map[string]any `json:"value"`
}

type usecase struct {
	repo repository.UserPreferencesRepository
}

func New(repo repository.UserPreferencesRepository) Usecase {
	return &usecase{repo: repo}
}

func (u *usecase) Get(ctx context.Context, workspaceID, userEmail, namespace string) (*entity.UserPreference, error) {
	if workspaceID == "" || userEmail == "" || namespace == "" {
		return nil, apperror.ValidationError("workspace_id, user_email and namespace required")
	}
	p, err := u.repo.Get(ctx, workspaceID, strings.ToLower(userEmail), namespace)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperror.NotFound("user_preference", "preference not found for namespace "+namespace)
	}
	return p, nil
}

func (u *usecase) List(ctx context.Context, workspaceID, userEmail string) ([]entity.UserPreference, error) {
	if workspaceID == "" || userEmail == "" {
		return nil, apperror.ValidationError("workspace_id and user_email required")
	}
	return u.repo.List(ctx, workspaceID, strings.ToLower(userEmail))
}

func (u *usecase) Upsert(ctx context.Context, req UpsertRequest) (*entity.UserPreference, error) {
	if req.WorkspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if strings.TrimSpace(req.UserEmail) == "" {
		return nil, apperror.ValidationError("user_email required")
	}
	if strings.TrimSpace(req.Namespace) == "" {
		return nil, apperror.ValidationError("namespace required")
	}
	if len(req.Namespace) > 128 {
		return nil, apperror.ValidationError("namespace must be <= 128 chars")
	}
	if req.Value == nil {
		req.Value = map[string]any{}
	}
	return u.repo.Upsert(ctx, &entity.UserPreference{
		WorkspaceID: req.WorkspaceID,
		UserEmail:   strings.ToLower(req.UserEmail),
		Namespace:   req.Namespace,
		Value:       req.Value,
	})
}

func (u *usecase) Delete(ctx context.Context, workspaceID, userEmail, namespace string) error {
	if workspaceID == "" || userEmail == "" || namespace == "" {
		return apperror.ValidationError("workspace_id, user_email and namespace required")
	}
	return u.repo.Delete(ctx, workspaceID, strings.ToLower(userEmail), namespace)
}
