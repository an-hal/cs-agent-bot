// Package auditworkspaceaccess records cross-workspace access events for
// compliance (PDP + 01-auth §2c).
package auditworkspaceaccess

import (
	"context"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

type Usecase interface {
	Record(ctx context.Context, req RecordRequest) (*entity.AuditWorkspaceAccess, error)
	List(ctx context.Context, filter entity.AuditWorkspaceAccessFilter) ([]entity.AuditWorkspaceAccess, int64, error)
}

type RecordRequest struct {
	WorkspaceID string `json:"workspace_id"`
	ActorEmail  string `json:"actor_email"`
	Kind        string `json:"access_kind"`
	Resource    string `json:"resource"`
	ResourceID  string `json:"resource_id"`
	IPAddress   string `json:"ip_address"`
	UserAgent   string `json:"user_agent"`
	Reason      string `json:"reason"`
}

type usecase struct {
	repo repository.AuditWorkspaceAccessRepository
}

func New(repo repository.AuditWorkspaceAccessRepository) Usecase {
	return &usecase{repo: repo}
}

func validKind(k string) bool {
	switch k {
	case entity.WorkspaceAccessKindRead, entity.WorkspaceAccessKindWrite, entity.WorkspaceAccessKindAdmin:
		return true
	}
	return false
}

func (u *usecase) Record(ctx context.Context, req RecordRequest) (*entity.AuditWorkspaceAccess, error) {
	if req.WorkspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if strings.TrimSpace(req.ActorEmail) == "" {
		return nil, apperror.ValidationError("actor_email required")
	}
	if !validKind(req.Kind) {
		return nil, apperror.ValidationError("access_kind must be read|write|admin")
	}
	return u.repo.Insert(ctx, &entity.AuditWorkspaceAccess{
		WorkspaceID: req.WorkspaceID,
		ActorEmail:  strings.ToLower(req.ActorEmail),
		Kind:        req.Kind,
		Resource:    req.Resource,
		ResourceID:  req.ResourceID,
		IPAddress:   req.IPAddress,
		UserAgent:   req.UserAgent,
		Reason:      req.Reason,
	})
}

func (u *usecase) List(ctx context.Context, filter entity.AuditWorkspaceAccessFilter) ([]entity.AuditWorkspaceAccess, int64, error) {
	if filter.WorkspaceID == "" {
		return nil, 0, apperror.ValidationError("workspace_id required")
	}
	return u.repo.List(ctx, filter)
}
