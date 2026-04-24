// Package reactivation manages dormant-client reactivation triggers.
// Triggers are per-workspace rules evaluated by a background job; when they
// match, a ReactivationEvent is recorded and a message template may be sent.
package reactivation

import (
	"context"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

type Usecase interface {
	UpsertTrigger(ctx context.Context, req UpsertTriggerRequest) (*entity.ReactivationTrigger, error)
	GetTrigger(ctx context.Context, workspaceID, id string) (*entity.ReactivationTrigger, error)
	ListTriggers(ctx context.Context, workspaceID string, activeOnly bool) ([]entity.ReactivationTrigger, error)
	DeleteTrigger(ctx context.Context, workspaceID, id string) error

	Reactivate(ctx context.Context, req ReactivateRequest) (*entity.ReactivationEvent, error)
	History(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ReactivationEvent, error)
}

type UpsertTriggerRequest struct {
	WorkspaceID  string `json:"workspace_id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Condition    string `json:"condition"`
	TemplateCode string `json:"template_code"`
	IsActive     *bool  `json:"is_active,omitempty"`
	ActorEmail   string `json:"-"`
}

// ReactivateRequest is an admin-triggered manual reactivation for one client.
// Backed by the "manual" trigger code unless a specific trigger is named.
type ReactivateRequest struct {
	WorkspaceID  string `json:"workspace_id"`
	MasterDataID string `json:"master_data_id"`
	TriggerCode  string `json:"trigger_code"`
	Note         string `json:"note"`
	ActorEmail   string `json:"-"`
}

// MutationRecorder is the optional hook used to record a master_data mutation
// whenever a reactivation event fires, so the /data-mutations feed reflects
// every touch on the client row (feat/08 §mutation coverage). Nil is legal.
type MutationRecorder interface {
	Append(ctx context.Context, m *entity.MasterDataMutation) error
}

type usecase struct {
	repo        repository.ReactivationRepository
	mutationLog MutationRecorder
}

func New(repo repository.ReactivationRepository) Usecase {
	return &usecase{repo: repo}
}

// NewWithMutation is like New but attaches a master_data mutation recorder
// so every reactivation event also writes to the mutation log — gives the
// activity-log feed full coverage (feat/08).
func NewWithMutation(repo repository.ReactivationRepository, rec MutationRecorder) Usecase {
	return &usecase{repo: repo, mutationLog: rec}
}

func (u *usecase) UpsertTrigger(ctx context.Context, req UpsertTriggerRequest) (*entity.ReactivationTrigger, error) {
	if req.WorkspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if strings.TrimSpace(req.Code) == "" {
		return nil, apperror.ValidationError("code required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperror.ValidationError("name required")
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	return u.repo.UpsertTrigger(ctx, &entity.ReactivationTrigger{
		WorkspaceID:  req.WorkspaceID,
		Code:         req.Code,
		Name:         req.Name,
		Description:  req.Description,
		Condition:    defaultString(req.Condition, "-"),
		TemplateCode: req.TemplateCode,
		IsActive:     active,
		CreatedBy:    strings.ToLower(req.ActorEmail),
	})
}

func (u *usecase) GetTrigger(ctx context.Context, workspaceID, id string) (*entity.ReactivationTrigger, error) {
	if workspaceID == "" || id == "" {
		return nil, apperror.ValidationError("workspace_id and id required")
	}
	out, err := u.repo.GetTrigger(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, apperror.NotFound("reactivation_trigger", id)
	}
	return out, nil
}

func (u *usecase) ListTriggers(ctx context.Context, workspaceID string, activeOnly bool) ([]entity.ReactivationTrigger, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	return u.repo.ListTriggers(ctx, workspaceID, activeOnly)
}

func (u *usecase) DeleteTrigger(ctx context.Context, workspaceID, id string) error {
	if workspaceID == "" || id == "" {
		return apperror.ValidationError("workspace_id and id required")
	}
	return u.repo.DeleteTrigger(ctx, workspaceID, id)
}

// Reactivate fires a reactivation event (manual or via a configured trigger).
// Rate-limit: at most 1 event of the same code per client per 30 days. This
// matches the FE spec's "don't spam the customer" guardrail.
func (u *usecase) Reactivate(ctx context.Context, req ReactivateRequest) (*entity.ReactivationEvent, error) {
	if req.WorkspaceID == "" || req.MasterDataID == "" {
		return nil, apperror.ValidationError("workspace_id and master_data_id required")
	}
	code := req.TriggerCode
	if code == "" {
		code = entity.ReactivationCodeManual
	}

	// Find the trigger row by code to link the event. A "manual" code is
	// auto-created on-demand so admins don't need to pre-configure it.
	triggers, err := u.repo.ListTriggers(ctx, req.WorkspaceID, false)
	if err != nil {
		return nil, err
	}
	var triggerID string
	for _, t := range triggers {
		if t.Code == code {
			triggerID = t.ID
			break
		}
	}
	if triggerID == "" {
		created, err := u.repo.UpsertTrigger(ctx, &entity.ReactivationTrigger{
			WorkspaceID: req.WorkspaceID,
			Code:        code,
			Name:        "Manual reactivation",
			Condition:   "-",
			IsActive:    true,
			CreatedBy:   strings.ToLower(req.ActorEmail),
		})
		if err != nil {
			return nil, err
		}
		triggerID = created.ID
	}

	// Rate-limit: 1 per client+code per 30d.
	n, err := u.repo.CountRecentForClient(ctx, req.WorkspaceID, req.MasterDataID, code, 30*24*time.Hour)
	if err != nil {
		return nil, err
	}
	if n > 0 && code != entity.ReactivationCodeManual {
		return nil, apperror.Conflict("reactivation for this client+code already fired within the last 30 days")
	}

	evt, err := u.repo.RecordEvent(ctx, &entity.ReactivationEvent{
		WorkspaceID:  req.WorkspaceID,
		TriggerID:    triggerID,
		MasterDataID: req.MasterDataID,
		Outcome:      entity.ReactivationOutcomeSent,
		Note:         req.Note,
	})
	if err != nil {
		return nil, err
	}
	// Mirror to master_data mutation log so activity feed shows every touch.
	if u.mutationLog != nil {
		_ = u.mutationLog.Append(ctx, &entity.MasterDataMutation{
			WorkspaceID:   req.WorkspaceID,
			MasterDataID:  req.MasterDataID,
			Action:        "reactivate_client",
			Source:        entity.MutationSourceReactivation,
			ActorEmail:    strings.ToLower(req.ActorEmail),
			ChangedFields: []string{"reactivation"},
			NewValues: map[string]any{
				"trigger_code":          code,
				"reactivation_event_id": evt.ID,
			},
			Note: req.Note,
		})
	}
	return evt, nil
}

func (u *usecase) History(ctx context.Context, workspaceID, masterDataID string, limit int) ([]entity.ReactivationEvent, error) {
	if workspaceID == "" || masterDataID == "" {
		return nil, apperror.ValidationError("workspace_id and master_data_id required")
	}
	return u.repo.ListEventsForMasterData(ctx, workspaceID, masterDataID, limit)
}

func defaultString(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
