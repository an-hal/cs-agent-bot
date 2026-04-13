// Package notification implements the cross-cutting notification hub used by
// other features to publish in-app, telegram, and email notifications.
package notification

import (
	"context"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// Usecase exposes notification operations.
type Usecase interface {
	Create(ctx context.Context, req CreateRequest) (*entity.Notification, error)
	List(ctx context.Context, filter entity.NotificationFilter) ([]entity.Notification, int64, error)
	CountUnread(ctx context.Context, workspaceID, recipientEmail string) (int64, error)
	MarkRead(ctx context.Context, id, recipientEmail string) error
	MarkAllRead(ctx context.Context, workspaceID, recipientEmail string) error
}

// CreateRequest is the input to publish a notification.
type CreateRequest struct {
	WorkspaceID    string `json:"workspace_id"`
	RecipientEmail string `json:"recipient_email"`
	Type           string `json:"type"`
	Icon           string `json:"icon"`
	Message        string `json:"message"`
	Href           string `json:"href"`
	SourceFeature  string `json:"source_feature"`
	SourceID       string `json:"source_id"`
}

type usecase struct {
	repo repository.NotificationRepository
}

// New constructs a notification usecase.
func New(repo repository.NotificationRepository) Usecase {
	return &usecase{repo: repo}
}

func (u *usecase) Create(ctx context.Context, req CreateRequest) (*entity.Notification, error) {
	if req.WorkspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if strings.TrimSpace(req.RecipientEmail) == "" {
		return nil, apperror.ValidationError("recipient_email required")
	}
	if strings.TrimSpace(req.Message) == "" {
		return nil, apperror.ValidationError("message required")
	}
	if req.Type == "" {
		req.Type = "info"
	}
	return u.repo.Create(ctx, &entity.Notification{
		WorkspaceID:    req.WorkspaceID,
		RecipientEmail: strings.ToLower(req.RecipientEmail),
		Type:           req.Type,
		Icon:           req.Icon,
		Message:        req.Message,
		Href:           req.Href,
		SourceFeature:  req.SourceFeature,
		SourceID:       req.SourceID,
	})
}

func (u *usecase) List(ctx context.Context, filter entity.NotificationFilter) ([]entity.Notification, int64, error) {
	if filter.WorkspaceID == "" || filter.RecipientEmail == "" {
		return nil, 0, apperror.ValidationError("workspace_id and recipient_email required")
	}
	return u.repo.List(ctx, filter)
}

func (u *usecase) CountUnread(ctx context.Context, workspaceID, recipientEmail string) (int64, error) {
	if workspaceID == "" || recipientEmail == "" {
		return 0, apperror.ValidationError("workspace_id and recipient_email required")
	}
	return u.repo.CountUnread(ctx, workspaceID, recipientEmail)
}

func (u *usecase) MarkRead(ctx context.Context, id, recipientEmail string) error {
	if id == "" {
		return apperror.ValidationError("id required")
	}
	return u.repo.MarkRead(ctx, id, recipientEmail)
}

func (u *usecase) MarkAllRead(ctx context.Context, workspaceID, recipientEmail string) error {
	if workspaceID == "" || recipientEmail == "" {
		return apperror.ValidationError("workspace_id and recipient_email required")
	}
	return u.repo.MarkAllRead(ctx, workspaceID, recipientEmail)
}
