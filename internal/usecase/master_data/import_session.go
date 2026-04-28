// Phase C: import sessions — wizard-side scratchpad that stores file + mapping
// + per-cell overrides so a maker can fix bad cells in the FE before
// submitting an approval. Without sessions, fixing a single typo in a 100-row
// xlsx required re-uploading the entire (corrected) file.
//
// Flow:
//   1. POST /import/sessions  → CreateSession(file, mapping)  → {id, preview, errors}
//   2. PATCH /import/sessions/{id}/cell  → PatchCell(row, target, value) → row's new errors
//   3. POST /import/sessions/{id}/submit → SubmitSession  → approval row
//   4. POST /approvals/{id}/apply  → existing apply path; reparses with the
//      session's mapping + overrides baked into the approval payload.

package master_data

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/xlsximport"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
)

// CreateSessionInput is what the handler passes to CreateSession after
// reading the multipart form.
type CreateSessionInput struct {
	FileName  string
	FileBytes []byte
	SheetName string
	Mode      ImportMode
	Mapping   map[string]string
}

// SessionPreview is the parse + dedup snapshot returned alongside session
// state. Reuses the existing ImportPreview shape so FE can render the same
// table for both the wizard and the legacy preview endpoint.
type SessionPreview struct {
	Session *entity.ImportSession `json:"session"`
	Preview *ImportPreview        `json:"preview"`
}

// PatchCellInput is the body for PATCH /import/sessions/{id}/cell.
type PatchCellInput struct {
	Row       int    `json:"row"`        // 1-based xlsx row number (header = 1, data starts at 2)
	TargetKey string `json:"target_key"` // canonical target field key
	Value     string `json:"value"`      // raw string; transforms run on read
}

// CreateSession persists a wizard session and returns the initial preview.
// The file is stored as base64 in import_sessions so subsequent reads can
// re-parse without the maker re-uploading. We deliberately keep file content
// out of the session HTTP responses (entity uses omitempty on FileB64).
func (u *usecase) CreateSession(ctx context.Context, workspaceID, actorEmail string, in CreateSessionInput) (*SessionPreview, error) {
	if u.sessionRepo == nil {
		return nil, apperror.BadRequest("import sessions not enabled (sessionRepo nil)")
	}
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if len(in.FileBytes) == 0 {
		return nil, apperror.ValidationError("file required")
	}
	if in.Mode == "" {
		in.Mode = ImportModeAddNew
	}
	if in.Mode != ImportModeAddNew && in.Mode != ImportModeUpdateExisting {
		return nil, apperror.ValidationError("invalid mode")
	}

	sess := &entity.ImportSession{
		WorkspaceID:   workspaceID,
		CreatedBy:     actorEmail,
		Status:        entity.ImportSessionStatusPending,
		FileName:      in.FileName,
		FileB64:       base64.StdEncoding.EncodeToString(in.FileBytes),
		SheetName:     strings.TrimSpace(in.SheetName),
		Mode:          string(in.Mode),
		Mapping:       in.Mapping,
		CellOverrides: map[string]map[string]string{},
	}
	saved, err := u.sessionRepo.Create(ctx, sess)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	preview, err := u.previewSession(ctx, saved)
	if err != nil {
		return nil, err
	}
	// Don't echo the full base64 blob back to FE.
	saved.FileB64 = ""
	return &SessionPreview{Session: saved, Preview: preview}, nil
}

// GetSession returns the current preview state for a session, re-parsing the
// stored file with the latest mapping + overrides.
func (u *usecase) GetSession(ctx context.Context, workspaceID, sessionID string) (*SessionPreview, error) {
	sess, err := u.loadSession(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	preview, err := u.previewSession(ctx, sess)
	if err != nil {
		return nil, err
	}
	sess.FileB64 = ""
	return &SessionPreview{Session: sess, Preview: preview}, nil
}

// PatchCell upserts a single cell override and returns the refreshed preview
// so FE can update its row without a separate GET.
func (u *usecase) PatchCell(ctx context.Context, workspaceID, sessionID string, in PatchCellInput) (*SessionPreview, error) {
	if in.Row < 2 {
		return nil, apperror.ValidationError("row must be >= 2 (row 1 is the header)")
	}
	if strings.TrimSpace(in.TargetKey) == "" {
		return nil, apperror.ValidationError("target_key required")
	}
	sess, err := u.loadSession(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.Status != entity.ImportSessionStatusPending {
		return nil, apperror.BadRequest("session is not pending (status=" + sess.Status + ")")
	}
	if sess.CellOverrides == nil {
		sess.CellOverrides = map[string]map[string]string{}
	}
	rowKey := strconv.Itoa(in.Row)
	rowOverrides, ok := sess.CellOverrides[rowKey]
	if !ok {
		rowOverrides = map[string]string{}
		sess.CellOverrides[rowKey] = rowOverrides
	}
	rowOverrides[in.TargetKey] = in.Value
	if err := u.sessionRepo.UpdateOverrides(ctx, workspaceID, sessionID, sess.CellOverrides); err != nil {
		return nil, err
	}

	preview, err := u.previewSession(ctx, sess)
	if err != nil {
		return nil, err
	}
	sess.FileB64 = ""
	return &SessionPreview{Session: sess, Preview: preview}, nil
}

// SubmitSession converts a pending session into a bulk_import_master_data
// approval. The approval payload carries file_b64 + mapping + overrides so
// ApplyApprovedImport reproduces the maker's intent verbatim.
func (u *usecase) SubmitSession(ctx context.Context, workspaceID, sessionID, actorEmail string) (*entity.ApprovalRequest, error) {
	sess, err := u.loadSession(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.Status != entity.ImportSessionStatusPending {
		return nil, apperror.BadRequest("session is not pending")
	}

	// Build a quick preview slice for the approval payload (FE shows the
	// first ~5 in the approval modal).
	rows, _, err := u.parseSessionRows(ctx, sess)
	if err != nil {
		return nil, err
	}
	previewRows := make([]map[string]any, 0, 5)
	for i, row := range rows {
		if i >= 5 {
			break
		}
		previewRows = append(previewRows, map[string]any{
			"company_id":   row.CompanyID,
			"company_name": row.CompanyName,
		})
	}

	ar, err := u.requestImportFromSession(ctx, sess, actorEmail, len(rows), previewRows)
	if err != nil {
		return nil, err
	}
	if err := u.sessionRepo.MarkSubmitted(ctx, workspaceID, sessionID, ar.ID); err != nil {
		// Approval is created; flag the session-state failure but don't unwind.
		return ar, fmt.Errorf("approval created (%s) but session mark-submitted failed: %w", ar.ID, err)
	}
	return ar, nil
}

// requestImportFromSession is RequestImportWithMapping plus the session's
// cell_overrides (so the apply step replays exactly what the maker saw).
func (u *usecase) requestImportFromSession(
	ctx context.Context,
	sess *entity.ImportSession,
	actorEmail string,
	rowCount int,
	preview []map[string]any,
) (*entity.ApprovalRequest, error) {
	payload := map[string]any{
		"file_name":      sess.FileName,
		"file_b64":       sess.FileB64,
		"mode":           sess.Mode,
		"row_count":      rowCount,
		"preview":        preview,
		"sheet_name":     sess.SheetName,
		"mapping":        sess.Mapping,
		"cell_overrides": sess.CellOverrides, // Apply path reads + replays these.
		"session_id":     sess.ID,
	}
	desc := fmt.Sprintf("Bulk import master data: %s (%d rows, mode=%s, session=%s)",
		sess.FileName, rowCount, sess.Mode, sess.ID)
	return u.approvalRepo.Create(ctx, &entity.ApprovalRequest{
		WorkspaceID: sess.WorkspaceID,
		RequestType: entity.ApprovalTypeBulkImport,
		Description: desc,
		Payload:     payload,
		MakerEmail:  actorEmail,
	})
}

// previewSession runs the parser with the session's mapping + overrides and
// builds an ImportPreview (dedup + cell errors).
func (u *usecase) previewSession(ctx context.Context, sess *entity.ImportSession) (*ImportPreview, error) {
	rows, cellErrs, err := u.parseSessionRows(ctx, sess)
	if err != nil {
		return nil, err
	}
	preview, err := u.PreviewImport(ctx, sess.WorkspaceID, rows, ImportMode(sess.Mode))
	if err != nil {
		return nil, err
	}
	if len(cellErrs) > 0 {
		preview.CellErrors = cellErrs
		preview.Invalid += len(cellErrs)
	}
	return preview, nil
}

// parseSessionRows decodes the session's file_b64 and runs the mapping
// parser with the stored overrides. Returns the parsed rows + cell errors.
func (u *usecase) parseSessionRows(ctx context.Context, sess *entity.ImportSession) ([]xlsximport.ClientImportRow, []xlsximport.CellError, error) {
	fileBytes, err := base64.StdEncoding.DecodeString(sess.FileB64)
	if err != nil {
		return nil, nil, apperror.BadRequest("session file decode: " + err.Error())
	}
	overrides := convertOverrides(sess.CellOverrides)
	return u.ParseImportRowsWithMapping(ctx, sess.WorkspaceID, bytes.NewReader(fileBytes),
		xlsximport.MappingParseOptions{
			SheetName: sess.SheetName,
			Mapping:   sess.Mapping,
			Overrides: overrides,
		})
}

func (u *usecase) loadSession(ctx context.Context, workspaceID, sessionID string) (*entity.ImportSession, error) {
	if u.sessionRepo == nil {
		return nil, apperror.BadRequest("import sessions not enabled")
	}
	if workspaceID == "" || sessionID == "" {
		return nil, apperror.ValidationError("workspace_id and session_id required")
	}
	sess, err := u.sessionRepo.GetByID(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, apperror.NotFound("import_session", sessionID)
	}
	return sess, nil
}

// convertOverrides reshapes the storage-friendly map[string]map[string]string
// (row_num as string key, JSONB-serializable) into the int-keyed shape the
// parser expects.
func convertOverrides(stored map[string]map[string]string) map[int]map[string]string {
	if len(stored) == 0 {
		return nil
	}
	out := make(map[int]map[string]string, len(stored))
	for k, v := range stored {
		n, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		out[n] = v
	}
	return out
}

// Marker so cmd/server's wireUsecases compiles when sessionRepo is nil:
// the Session* methods all guard on u.sessionRepo == nil.
var _ repository.ImportSessionRepository = (repository.ImportSessionRepository)(nil)
