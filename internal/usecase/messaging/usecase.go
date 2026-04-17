// Package messaging implements CRUD + preview/render for the multi-tenant
// message and email template system defined in feat/05-messaging.
package messaging

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/htmlsanitize"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/lib/pq"
	"github.com/rs/zerolog"
)

// variableRe matches [Variable_Name] placeholders used by the spec.
// Leading char must be A-Za-z or _, subsequent chars A-Za-z0-9_.
var variableRe = regexp.MustCompile(`\[([A-Za-z_][A-Za-z0-9_]*)\]`)

// Usecase is the messaging feature usecase.
type Usecase interface {
	// Message templates
	ListMessageTemplates(ctx context.Context, workspaceID string, filter entity.MessageTemplateFilter) ([]entity.MessageTemplate, error)
	GetMessageTemplate(ctx context.Context, workspaceID, id string) (*entity.MessageTemplate, error)
	CreateMessageTemplate(ctx context.Context, editor string, t *entity.MessageTemplate) (*entity.MessageTemplate, error)
	UpdateMessageTemplate(ctx context.Context, editor string, t *entity.MessageTemplate) (*entity.MessageTemplate, []string, error)
	DeleteMessageTemplate(ctx context.Context, editor, workspaceID, id string) error

	// Email templates
	ListEmailTemplates(ctx context.Context, workspaceID string, filter entity.EmailTemplateFilter) ([]entity.EmailTemplate, error)
	GetEmailTemplate(ctx context.Context, workspaceID, id string) (*entity.EmailTemplate, error)
	CreateEmailTemplate(ctx context.Context, editor string, t *entity.EmailTemplate) (*entity.EmailTemplate, error)
	UpdateEmailTemplate(ctx context.Context, editor string, t *entity.EmailTemplate) (*entity.EmailTemplate, []string, error)
	DeleteEmailTemplate(ctx context.Context, editor, workspaceID, id string) error

	// Edit logs
	ListEditLogs(ctx context.Context, workspaceID string, filter entity.TemplateEditLogFilter) ([]entity.TemplateEditLog, error)

	// Variables
	ListVariables(ctx context.Context, workspaceID string) ([]entity.TemplateVariable, error)

	// Preview
	Preview(ctx context.Context, workspaceID string, req PreviewRequest) (*PreviewResult, error)
}

// PreviewRequest carries inputs for the /templates/preview endpoint.
type PreviewRequest struct {
	TemplateType string            `json:"template_type"`
	TemplateID   string            `json:"template_id"`
	SampleData   map[string]string `json:"sample_data"`
}

// PreviewResult is the rendered output for preview and render endpoints.
type PreviewResult struct {
	TemplateType      string `json:"template_type"`
	Rendered          string `json:"rendered,omitempty"`
	Subject           string `json:"subject,omitempty"`
	BodyHTML          string `json:"body_html,omitempty"`
	MissingVariables  []string `json:"missing_variables"`
}

type usecase struct {
	msgRepo   repository.MessageTemplateRepository
	emailRepo repository.EmailTemplateRepository
	varRepo   repository.TemplateVariableRepository
	logRepo   repository.TemplateEditLogRepository
	logger    zerolog.Logger
}

// New constructs the messaging usecase.
func New(
	msgRepo repository.MessageTemplateRepository,
	emailRepo repository.EmailTemplateRepository,
	varRepo repository.TemplateVariableRepository,
	logRepo repository.TemplateEditLogRepository,
	logger zerolog.Logger,
) Usecase {
	return &usecase{
		msgRepo:   msgRepo,
		emailRepo: emailRepo,
		varRepo:   varRepo,
		logRepo:   logRepo,
		logger:    logger,
	}
}

// ExtractVariables returns the sorted, unique list of [Variable_Name]
// placeholders found in the given text.
func ExtractVariables(texts ...string) []string {
	seen := make(map[string]struct{})
	for _, t := range texts {
		for _, m := range variableRe.FindAllStringSubmatch(t, -1) {
			seen[m[1]] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Render substitutes [Variable_Name] placeholders using sampleData and
// reports any placeholders left unresolved.
func Render(text string, sampleData map[string]string) (string, []string) {
	missing := map[string]struct{}{}
	rendered := variableRe.ReplaceAllStringFunc(text, func(match string) string {
		key := match[1 : len(match)-1]
		if v, ok := sampleData[key]; ok {
			return v
		}
		missing[key] = struct{}{}
		return match
	})
	out := make([]string, 0, len(missing))
	for k := range missing {
		out = append(out, k)
	}
	sort.Strings(out)
	return rendered, out
}

// ============================================================================
// Message templates
// ============================================================================

func (u *usecase) ListMessageTemplates(ctx context.Context, workspaceID string, filter entity.MessageTemplateFilter) ([]entity.MessageTemplate, error) {
	out, err := u.msgRepo.List(ctx, workspaceID, filter)
	if err != nil {
		return nil, apperror.WrapInternal(u.logger, err, "list message templates")
	}
	if out == nil {
		out = []entity.MessageTemplate{}
	}
	return out, nil
}

func (u *usecase) GetMessageTemplate(ctx context.Context, workspaceID, id string) (*entity.MessageTemplate, error) {
	t, err := u.msgRepo.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, apperror.WrapInternal(u.logger, err, "get message template")
	}
	if t == nil {
		return nil, apperror.NotFound("template", "Template not found")
	}
	return t, nil
}

func (u *usecase) CreateMessageTemplate(ctx context.Context, editor string, t *entity.MessageTemplate) (*entity.MessageTemplate, error) {
	if t.ID == "" {
		return nil, apperror.ValidationError("id is required")
	}
	if t.Channel == "" {
		t.Channel = entity.MsgTplChannelWhatsApp
	}
	if t.Role == "" {
		t.Role = entity.MsgTplRoleAE
	}
	if len(t.Variables) == 0 {
		t.Variables = ExtractVariables(t.Message)
	}
	now := time.Now().UTC()
	t.UpdatedAt = &now
	t.UpdatedBy = &editor

	created, err := u.msgRepo.Create(ctx, t)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, apperror.Conflict(fmt.Sprintf("Template ID %s already exists in this workspace", t.ID))
		}
		return nil, apperror.WrapInternal(u.logger, err, "create message template")
	}

	_, _ = u.logRepo.Append(ctx, &entity.TemplateEditLog{
		WorkspaceID:  t.WorkspaceID,
		TemplateID:   t.ID,
		TemplateType: entity.TemplateTypeMessage,
		Field:        entity.TemplateEditFieldCreated,
		NewValue:     strPtr(created.Message),
		EditedBy:     editor,
	})
	return created, nil
}

func (u *usecase) UpdateMessageTemplate(ctx context.Context, editor string, patch *entity.MessageTemplate) (*entity.MessageTemplate, []string, error) {
	existing, err := u.msgRepo.Get(ctx, patch.WorkspaceID, patch.ID)
	if err != nil {
		return nil, nil, apperror.WrapInternal(u.logger, err, "load existing message template")
	}
	if existing == nil {
		return nil, nil, apperror.NotFound("template", "Template not found")
	}

	// Merge patch into existing — the handler already loaded non-zero fields into `patch`.
	merged := *existing
	mergeMessageTemplate(&merged, patch)
	if len(patch.Variables) == 0 {
		merged.Variables = ExtractVariables(merged.Message)
	} else {
		merged.Variables = patch.Variables
	}
	merged.UpdatedBy = &editor

	changed := diffMessageTemplate(existing, &merged)
	if len(changed) == 0 {
		return existing, nil, nil
	}

	updated, err := u.msgRepo.Update(ctx, &merged)
	if err != nil {
		return nil, nil, apperror.WrapInternal(u.logger, err, "update message template")
	}

	// Append edit logs for every changed field (best-effort; logging errors are non-fatal).
	for _, f := range changed {
		old, new := fieldValue(existing, f), fieldValue(updated, f)
		if _, appendErr := u.logRepo.Append(ctx, &entity.TemplateEditLog{
			WorkspaceID:  existing.WorkspaceID,
			TemplateID:   existing.ID,
			TemplateType: entity.TemplateTypeMessage,
			Field:        f,
			OldValue:     old,
			NewValue:     new,
			EditedBy:     editor,
		}); appendErr != nil {
			u.logger.Warn().Err(appendErr).Str("template_id", existing.ID).Str("field", f).Msg("append edit log failed")
		}
	}
	return updated, changed, nil
}

func (u *usecase) DeleteMessageTemplate(ctx context.Context, editor, workspaceID, id string) error {
	if err := u.msgRepo.Delete(ctx, workspaceID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.NotFound("template", "Template not found")
		}
		return apperror.WrapInternal(u.logger, err, "delete message template")
	}
	_, _ = u.logRepo.Append(ctx, &entity.TemplateEditLog{
		WorkspaceID:  workspaceID,
		TemplateID:   id,
		TemplateType: entity.TemplateTypeMessage,
		Field:        entity.TemplateEditFieldDeleted,
		EditedBy:     editor,
	})
	return nil
}

// ============================================================================
// Email templates
// ============================================================================

func (u *usecase) ListEmailTemplates(ctx context.Context, workspaceID string, filter entity.EmailTemplateFilter) ([]entity.EmailTemplate, error) {
	out, err := u.emailRepo.List(ctx, workspaceID, filter)
	if err != nil {
		return nil, apperror.WrapInternal(u.logger, err, "list email templates")
	}
	if out == nil {
		out = []entity.EmailTemplate{}
	}
	return out, nil
}

func (u *usecase) GetEmailTemplate(ctx context.Context, workspaceID, id string) (*entity.EmailTemplate, error) {
	t, err := u.emailRepo.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, apperror.WrapInternal(u.logger, err, "get email template")
	}
	if t == nil {
		return nil, apperror.NotFound("template", "Template not found")
	}
	return t, nil
}

func (u *usecase) CreateEmailTemplate(ctx context.Context, editor string, t *entity.EmailTemplate) (*entity.EmailTemplate, error) {
	if t.ID == "" {
		return nil, apperror.ValidationError("id is required")
	}
	if t.Role == "" {
		t.Role = entity.MsgTplRoleAE
	}
	if t.Status == "" {
		t.Status = entity.EmailTplStatusDraft
	}
	t.BodyHTML = htmlsanitize.SanitizeEmailHTML(t.BodyHTML)
	if len(t.Variables) == 0 {
		t.Variables = ExtractVariables(t.Subject, t.BodyHTML)
	}
	now := time.Now().UTC()
	t.UpdatedAt = &now
	t.UpdatedBy = &editor

	created, err := u.emailRepo.Create(ctx, t)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, apperror.Conflict(fmt.Sprintf("Template ID %s already exists in this workspace", t.ID))
		}
		return nil, apperror.WrapInternal(u.logger, err, "create email template")
	}
	_, _ = u.logRepo.Append(ctx, &entity.TemplateEditLog{
		WorkspaceID:  t.WorkspaceID,
		TemplateID:   t.ID,
		TemplateType: entity.TemplateTypeEmail,
		Field:        entity.TemplateEditFieldCreated,
		NewValue:     strPtr(created.Subject),
		EditedBy:     editor,
	})
	return created, nil
}

func (u *usecase) UpdateEmailTemplate(ctx context.Context, editor string, patch *entity.EmailTemplate) (*entity.EmailTemplate, []string, error) {
	existing, err := u.emailRepo.Get(ctx, patch.WorkspaceID, patch.ID)
	if err != nil {
		return nil, nil, apperror.WrapInternal(u.logger, err, "load existing email template")
	}
	if existing == nil {
		return nil, nil, apperror.NotFound("template", "Template not found")
	}
	merged := *existing
	mergeEmailTemplate(&merged, patch)
	merged.BodyHTML = htmlsanitize.SanitizeEmailHTML(merged.BodyHTML)
	if len(patch.Variables) == 0 {
		merged.Variables = ExtractVariables(merged.Subject, merged.BodyHTML)
	} else {
		merged.Variables = patch.Variables
	}
	merged.UpdatedBy = &editor

	changed := diffEmailTemplate(existing, &merged)
	if len(changed) == 0 {
		return existing, nil, nil
	}
	updated, err := u.emailRepo.Update(ctx, &merged)
	if err != nil {
		return nil, nil, apperror.WrapInternal(u.logger, err, "update email template")
	}
	for _, f := range changed {
		old, new := emailFieldValue(existing, f), emailFieldValue(updated, f)
		if _, appendErr := u.logRepo.Append(ctx, &entity.TemplateEditLog{
			WorkspaceID:  existing.WorkspaceID,
			TemplateID:   existing.ID,
			TemplateType: entity.TemplateTypeEmail,
			Field:        f,
			OldValue:     old,
			NewValue:     new,
			EditedBy:     editor,
		}); appendErr != nil {
			u.logger.Warn().Err(appendErr).Str("template_id", existing.ID).Str("field", f).Msg("append edit log failed")
		}
	}
	return updated, changed, nil
}

func (u *usecase) DeleteEmailTemplate(ctx context.Context, editor, workspaceID, id string) error {
	if err := u.emailRepo.Delete(ctx, workspaceID, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.NotFound("template", "Template not found")
		}
		return apperror.WrapInternal(u.logger, err, "delete email template")
	}
	_, _ = u.logRepo.Append(ctx, &entity.TemplateEditLog{
		WorkspaceID:  workspaceID,
		TemplateID:   id,
		TemplateType: entity.TemplateTypeEmail,
		Field:        entity.TemplateEditFieldDeleted,
		EditedBy:     editor,
	})
	return nil
}

// ============================================================================
// Edit logs, variables, preview
// ============================================================================

func (u *usecase) ListEditLogs(ctx context.Context, workspaceID string, filter entity.TemplateEditLogFilter) ([]entity.TemplateEditLog, error) {
	out, err := u.logRepo.List(ctx, workspaceID, filter)
	if err != nil {
		return nil, apperror.WrapInternal(u.logger, err, "list edit logs")
	}
	if out == nil {
		out = []entity.TemplateEditLog{}
	}
	return out, nil
}

func (u *usecase) ListVariables(ctx context.Context, workspaceID string) ([]entity.TemplateVariable, error) {
	out, err := u.varRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, apperror.WrapInternal(u.logger, err, "list template variables")
	}
	if out == nil {
		out = []entity.TemplateVariable{}
	}
	return out, nil
}

func (u *usecase) Preview(ctx context.Context, workspaceID string, req PreviewRequest) (*PreviewResult, error) {
	switch req.TemplateType {
	case entity.TemplateTypeMessage:
		t, err := u.msgRepo.Get(ctx, workspaceID, req.TemplateID)
		if err != nil {
			return nil, apperror.WrapInternal(u.logger, err, "load message template")
		}
		if t == nil {
			return nil, apperror.NotFound("template", "Template not found")
		}
		rendered, missing := Render(t.Message, req.SampleData)
		return &PreviewResult{
			TemplateType:     entity.TemplateTypeMessage,
			Rendered:         rendered,
			MissingVariables: missing,
		}, nil
	case entity.TemplateTypeEmail:
		t, err := u.emailRepo.Get(ctx, workspaceID, req.TemplateID)
		if err != nil {
			return nil, apperror.WrapInternal(u.logger, err, "load email template")
		}
		if t == nil {
			return nil, apperror.NotFound("template", "Template not found")
		}
		subject, subMissing := Render(t.Subject, req.SampleData)
		body, bodyMissing := Render(t.BodyHTML, req.SampleData)
		missing := mergeSortedUnique(subMissing, bodyMissing)
		return &PreviewResult{
			TemplateType:     entity.TemplateTypeEmail,
			Subject:          subject,
			BodyHTML:         body,
			MissingVariables: missing,
		}, nil
	default:
		return nil, apperror.ValidationError("template_type must be 'message' or 'email'")
	}
}

// ============================================================================
// helpers
// ============================================================================

func strPtr(s string) *string { return &s }

func mergeMessageTemplate(dst, src *entity.MessageTemplate) {
	if src.TriggerID != "" {
		dst.TriggerID = src.TriggerID
	}
	if src.Phase != "" {
		dst.Phase = src.Phase
	}
	if src.PhaseLabel != "" {
		dst.PhaseLabel = src.PhaseLabel
	}
	if src.Channel != "" {
		dst.Channel = src.Channel
	}
	if src.Role != "" {
		dst.Role = src.Role
	}
	if src.Category != "" {
		dst.Category = src.Category
	}
	if src.Action != "" {
		dst.Action = src.Action
	}
	if src.Timing != "" {
		dst.Timing = src.Timing
	}
	if src.Condition != "" {
		dst.Condition = src.Condition
	}
	if src.Message != "" {
		dst.Message = src.Message
	}
	if src.StopIf != nil {
		dst.StopIf = src.StopIf
	}
	if src.SentFlag != "" {
		dst.SentFlag = src.SentFlag
	}
	if src.Priority != nil {
		dst.Priority = src.Priority
	}
}

func diffMessageTemplate(a, b *entity.MessageTemplate) []string {
	var out []string
	if a.TriggerID != b.TriggerID {
		out = append(out, "trigger_id")
	}
	if a.Phase != b.Phase {
		out = append(out, "phase")
	}
	if a.PhaseLabel != b.PhaseLabel {
		out = append(out, "phase_label")
	}
	if a.Channel != b.Channel {
		out = append(out, "channel")
	}
	if a.Role != b.Role {
		out = append(out, "role")
	}
	if a.Category != b.Category {
		out = append(out, "category")
	}
	if a.Action != b.Action {
		out = append(out, "action")
	}
	if a.Timing != b.Timing {
		out = append(out, "timing")
	}
	if a.Condition != b.Condition {
		out = append(out, "condition")
	}
	if a.Message != b.Message {
		out = append(out, "message")
	}
	if !sliceEqual(a.Variables, b.Variables) {
		out = append(out, "variables")
	}
	if strPtrEq(a.StopIf, b.StopIf) == false {
		out = append(out, "stop_if")
	}
	if a.SentFlag != b.SentFlag {
		out = append(out, "sent_flag")
	}
	if strPtrEq(a.Priority, b.Priority) == false {
		out = append(out, "priority")
	}
	return out
}

func fieldValue(t *entity.MessageTemplate, f string) *string {
	switch f {
	case "trigger_id":
		return strPtr(t.TriggerID)
	case "phase":
		return strPtr(t.Phase)
	case "phase_label":
		return strPtr(t.PhaseLabel)
	case "channel":
		return strPtr(t.Channel)
	case "role":
		return strPtr(t.Role)
	case "category":
		return strPtr(t.Category)
	case "action":
		return strPtr(t.Action)
	case "timing":
		return strPtr(t.Timing)
	case "condition":
		return strPtr(t.Condition)
	case "message":
		return strPtr(t.Message)
	case "variables":
		return strPtr(strings.Join(t.Variables, ","))
	case "stop_if":
		return t.StopIf
	case "sent_flag":
		return strPtr(t.SentFlag)
	case "priority":
		return t.Priority
	}
	return nil
}

func mergeEmailTemplate(dst, src *entity.EmailTemplate) {
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.Role != "" {
		dst.Role = src.Role
	}
	if src.Category != "" {
		dst.Category = src.Category
	}
	if src.Status != "" {
		dst.Status = src.Status
	}
	if src.Subject != "" {
		dst.Subject = src.Subject
	}
	if src.BodyHTML != "" {
		dst.BodyHTML = src.BodyHTML
	}
}

func diffEmailTemplate(a, b *entity.EmailTemplate) []string {
	var out []string
	if a.Name != b.Name {
		out = append(out, "name")
	}
	if a.Role != b.Role {
		out = append(out, "role")
	}
	if a.Category != b.Category {
		out = append(out, "category")
	}
	if a.Status != b.Status {
		out = append(out, "status")
	}
	if a.Subject != b.Subject {
		out = append(out, "subject")
	}
	if a.BodyHTML != b.BodyHTML {
		out = append(out, "body_html")
	}
	if !sliceEqual(a.Variables, b.Variables) {
		out = append(out, "variables")
	}
	return out
}

func emailFieldValue(t *entity.EmailTemplate, f string) *string {
	switch f {
	case "name":
		return strPtr(t.Name)
	case "role":
		return strPtr(t.Role)
	case "category":
		return strPtr(t.Category)
	case "status":
		return strPtr(t.Status)
	case "subject":
		return strPtr(t.Subject)
	case "body_html":
		return strPtr(t.BodyHTML)
	case "variables":
		return strPtr(strings.Join(t.Variables, ","))
	}
	return nil
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func strPtrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func mergeSortedUnique(a, b []string) []string {
	seen := map[string]struct{}{}
	for _, s := range a {
		seen[s] = struct{}{}
	}
	for _, s := range b {
		seen[s] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func isDuplicateKey(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code.Name() == "unique_violation"
	}
	return strings.Contains(err.Error(), "unique_violation") || strings.Contains(err.Error(), "duplicate key")
}
