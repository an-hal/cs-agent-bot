package master_data

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/xlsxexport"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/xlsximport"
	"github.com/xuri/excelize/v2"
)

// ImportMode controls how Import handles existing rows.
type ImportMode string

const (
	// ImportModeAddNew skips rows whose company_id already exists.
	ImportModeAddNew ImportMode = "add_new"
	// ImportModeUpdateExisting only updates rows whose company_id already exists.
	ImportModeUpdateExisting ImportMode = "update_existing"
)

// ImportResult is what an import job ultimately produces.
type ImportResult struct {
	Imported int                 `json:"imported"`
	Skipped  int                 `json:"skipped"`
	Errors   []ImportRowError    `json:"errors"`
	Preview  []entity.MasterData `json:"preview"`
}

// ImportRowError captures one bad row.
type ImportRowError struct {
	Row       int    `json:"row"`
	Error     string `json:"error"`
	CompanyID string `json:"company_id,omitempty"`
}

// ImportPreview summarizes a dedup-preview run — parse + lookup, no writes.
// FE shows this in a modal before asking the user to confirm the actual import.
type ImportPreview struct {
	Mode       ImportMode         `json:"mode"`
	TotalRows  int                `json:"total_rows"`
	New        int                `json:"new"`
	Duplicates int                `json:"duplicates"`
	Invalid    int                `json:"invalid"`
	Rows       []ImportPreviewRow `json:"rows"`
}

// ImportPreviewRow is one line in the preview table.
type ImportPreviewRow struct {
	Row         int    `json:"row"`
	Status      string `json:"status"` // "new" | "duplicate" | "invalid"
	CompanyID   string `json:"company_id,omitempty"`
	CompanyName string `json:"company_name,omitempty"`
	ExistingID  string `json:"existing_id,omitempty"`
	Error       string `json:"error,omitempty"`
}

// RequestImport creates a checker-maker approval row for an import. The actual
// processing happens after approval (via ApplyApprovedImport).
func (u *usecase) RequestImport(
	ctx context.Context,
	workspaceID, actorEmail, fileName string,
	mode ImportMode,
	rowCount int,
	preview []map[string]any,
	fileRef string,
) (*entity.ApprovalRequest, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if mode != ImportModeAddNew && mode != ImportModeUpdateExisting {
		return nil, apperror.ValidationError("invalid import mode")
	}
	payload := map[string]any{
		"file_name": fileName,
		"file_ref":  fileRef,
		"mode":      string(mode),
		"row_count": rowCount,
		"preview":   preview,
	}
	desc := fmt.Sprintf("Bulk import master data: %s (%d rows, mode=%s)", fileName, rowCount, mode)
	return u.approvalRepo.Create(ctx, &entity.ApprovalRequest{
		WorkspaceID: workspaceID,
		RequestType: entity.ApprovalTypeBulkImport,
		Description: desc,
		Payload:     payload,
		MakerEmail:  actorEmail,
	})
}

// ParseImportRows parses an xlsx into raw rows for preview / dry-run inspection.
// The actual master_data inserts are deferred until approval.
func ParseImportRows(r io.Reader) ([]xlsximport.ClientImportRow, []xlsximport.ParseError, error) {
	return xlsximport.ParseClientSheet(r)
}

// ApplyApprovedImport applies an approved bulk import.
func (u *usecase) ApplyApprovedImport(
	ctx context.Context,
	workspaceID, approvalID, checkerEmail string,
	rows []xlsximport.ClientImportRow,
) (*ImportResult, error) {
	ar, err := u.approvalRepo.GetByID(ctx, workspaceID, approvalID)
	if err != nil {
		return nil, err
	}
	if ar == nil || ar.RequestType != entity.ApprovalTypeBulkImport {
		return nil, apperror.NotFound("bulk_import approval", "")
	}
	if ar.Status != entity.ApprovalStatusPending {
		return nil, apperror.BadRequest("approval is not pending")
	}
	mode := ImportModeAddNew
	if v, ok := ar.Payload["mode"].(string); ok {
		mode = ImportMode(v)
	}

	// One-shot dedup lookup. For add_new we use it to skip duplicates; for
	// update_existing we use it to route each row to Patch by the resolved
	// master_data.id instead of the business key company_id.
	companyIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.CompanyID != "" {
			companyIDs = append(companyIDs, strings.TrimSpace(row.CompanyID))
		}
	}
	existing, err := u.repo.ExistingCompanyIDs(ctx, workspaceID, companyIDs)
	if err != nil {
		return nil, fmt.Errorf("bulk import dedup lookup: %w", err)
	}

	res := &ImportResult{Errors: []ImportRowError{}}
	for i, row := range rows {
		if row.CompanyID == "" || row.CompanyName == "" {
			res.Errors = append(res.Errors, ImportRowError{
				Row: i + 2, Error: "missing company_id or company_name", CompanyID: row.CompanyID,
			})
			continue
		}
		cid := strings.TrimSpace(row.CompanyID)
		existingID, isDup := existing[cid]
		req := CreateRequest{
			CompanyID:       cid,
			CompanyName:     row.CompanyName,
			PICName:         row.PICName,
			PICRole:         row.PICRole,
			PICWA:           row.PICWA,
			PICEmail:        row.PICEmail,
			OwnerName:       row.OwnerName,
			OwnerWA:         row.OwnerWA,
			OwnerTelegramID: row.OwnerTelegramID,
			ContractStart:   ptrTime(row.ContractStart),
			ContractEnd:     ptrTime(row.ContractEnd),
			ContractMonths:  row.ContractMonths,
			PaymentTerms:    row.PaymentTerms,
			PaymentStatus:   row.PaymentStatus,
			FinalPrice:      int64(row.FinalPrice),
			Notes:           row.Notes,
		}

		if mode == ImportModeUpdateExisting {
			if !isDup {
				// Row not found — in update_existing mode we skip rather than insert.
				res.Skipped++
				continue
			}
			patch := PatchRequest{
				CompanyName:     &req.CompanyName,
				PICName:         ptrIfNonEmpty(req.PICName),
				PICRole:         ptrIfNonEmpty(req.PICRole),
				PICWA:           ptrIfNonEmpty(req.PICWA),
				PICEmail:        ptrIfNonEmpty(req.PICEmail),
				OwnerName:       ptrIfNonEmpty(req.OwnerName),
				OwnerWA:         ptrIfNonEmpty(req.OwnerWA),
				OwnerTelegramID: ptrIfNonEmpty(req.OwnerTelegramID),
				ContractStart:   req.ContractStart,
				ContractEnd:     req.ContractEnd,
				ContractMonths:  ptrIfNonZeroInt(req.ContractMonths),
				PaymentTerms:    ptrIfNonEmpty(req.PaymentTerms),
				PaymentStatus:   ptrIfNonEmpty(req.PaymentStatus),
				FinalPrice:      ptrIfNonZero64(req.FinalPrice),
				Notes:           ptrIfNonEmpty(req.Notes),
			}
			if _, _, err := u.Patch(ctx, workspaceID, existingID, checkerEmail, WriteContextDashboardUser, patch); err != nil {
				res.Errors = append(res.Errors, ImportRowError{
					Row: i + 2, Error: err.Error(), CompanyID: cid,
				})
				continue
			}
			res.Imported++ // "imported" here = row touched; FE renders as updated.
			continue
		}

		// add_new mode — skip duplicates cleanly.
		if isDup {
			res.Skipped++
			continue
		}
		if _, err := u.Create(ctx, workspaceID, checkerEmail, req); err != nil {
			res.Errors = append(res.Errors, ImportRowError{
				Row: i + 2, Error: err.Error(), CompanyID: cid,
			})
			continue
		}
		res.Imported++
	}

	_ = u.approvalRepo.UpdateStatus(ctx, workspaceID, approvalID, entity.ApprovalStatusApproved, checkerEmail, "")
	return res, nil
}

// Export writes all master_data rows to the writer as an XLSX.
func (u *usecase) Export(ctx context.Context, workspaceID string, w io.Writer) error {
	if workspaceID == "" {
		return apperror.ValidationError("workspace_id required")
	}
	defs, err := u.cfdRepo.List(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("load defs: %w", err)
	}
	sort.Slice(defs, func(i, j int) bool {
		if defs[i].SortOrder != defs[j].SortOrder {
			return defs[i].SortOrder < defs[j].SortOrder
		}
		return defs[i].FieldKey < defs[j].FieldKey
	})

	rows, _, err := u.repo.List(ctx, entity.MasterDataFilter{
		WorkspaceIDs: []string{workspaceID},
		Limit:        200,
	})
	if err != nil {
		return err
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "MasterData"
	f.SetSheetName(f.GetSheetName(0), sheet)

	headers := CoreExportHeaders()
	for _, d := range defs {
		headers = append(headers, "[Custom] "+d.FieldLabel)
	}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		_ = f.SetCellValue(sheet, cell, xlsxexport.SanitizeCell(h))
	}

	for i, m := range rows {
		values := coreExportValues(&m)
		for _, d := range defs {
			values = append(values, fmt.Sprint(m.CustomFields[d.FieldKey]))
		}
		for col, v := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, i+2)
			_ = f.SetCellValue(sheet, cell, xlsxexport.SanitizeCell(v))
		}
	}
	return f.Write(w)
}

// Template writes a 2-sheet xlsx template (Template + Reference).
func (u *usecase) Template(ctx context.Context, workspaceID string, w io.Writer) error {
	if workspaceID == "" {
		return apperror.ValidationError("workspace_id required")
	}
	defs, err := u.cfdRepo.List(ctx, workspaceID)
	if err != nil {
		return err
	}
	f := excelize.NewFile()
	defer f.Close()
	tplSheet := "Template"
	refSheet := "Reference"
	f.SetSheetName(f.GetSheetName(0), tplSheet)
	if _, err := f.NewSheet(refSheet); err != nil {
		return fmt.Errorf("new ref sheet: %w", err)
	}

	headers := CoreExportHeaders()
	for _, d := range defs {
		headers = append(headers, "[Custom] "+d.FieldLabel)
	}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		_ = f.SetCellValue(tplSheet, cell, h)
	}
	example := []any{"DE-EXAMPLE", "PT Example", "LEAD", "John", "PIC Role", "628100000000", "ex@example.com"}
	for col, v := range example {
		cell, _ := excelize.CoordinatesToCellName(col+1, 2)
		_ = f.SetCellValue(tplSheet, cell, v)
	}

	row := 1
	for _, d := range defs {
		if d.FieldType != entity.FieldTypeSelect {
			continue
		}
		opts := d.SelectOptions()
		cell, _ := excelize.CoordinatesToCellName(1, row)
		_ = f.SetCellValue(refSheet, cell, d.FieldLabel)
		for i, o := range opts {
			cell, _ := excelize.CoordinatesToCellName(2+i, row)
			_ = f.SetCellValue(refSheet, cell, o)
		}
		row++
	}
	return f.Write(w)
}

// CoreExportHeaders returns the canonical core column headers (exported for tests).
func CoreExportHeaders() []string {
	return []string{
		"Company ID", "Company Name", "Stage", "PIC Name", "PIC Role", "PIC WA", "PIC Email",
		"Owner Name", "Owner WA", "Bot Active", "Blacklisted", "Risk Flag",
		"Contract Start", "Contract End", "Contract Months",
		"Payment Status", "Payment Terms", "Final Price", "Notes",
	}
}

func coreExportValues(m *entity.MasterData) []any {
	return []any{
		m.CompanyID, m.CompanyName, m.Stage, m.PICName, m.PICRole, m.PICWA, m.PICEmail,
		m.OwnerName, m.OwnerWA, boolStr(m.BotActive), boolStr(m.Blacklisted), m.RiskFlag,
		formatNullableTime(m.ContractStart), formatNullableTime(m.ContractEnd), m.ContractMonths,
		m.PaymentStatus, m.PaymentTerms, m.FinalPrice, m.Notes,
	}
}

func boolStr(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func formatNullableTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func ptrTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// ptrIfNonEmpty returns a pointer to s only when s is non-empty. Used to keep
// bulk-import update patches "set only what changed" so empty cells don't
// clobber existing data.
func ptrIfNonEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func ptrIfNonZeroInt(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func ptrIfNonZero64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

// PreviewImport runs a dry-run over parsed rows: normalizes company_id,
// checks for duplicates in master_data, and reports per-row status without
// touching the DB. FE displays this in a confirmation modal so the user knows
// exactly how many rows will add vs update vs skip before signing off.
func (u *usecase) PreviewImport(ctx context.Context, workspaceID string, rows []xlsximport.ClientImportRow, mode ImportMode) (*ImportPreview, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	if mode != ImportModeAddNew && mode != ImportModeUpdateExisting {
		return nil, apperror.ValidationError("invalid import mode: must be add_new or update_existing")
	}

	preview := &ImportPreview{
		Mode:      mode,
		TotalRows: len(rows),
		Rows:      make([]ImportPreviewRow, 0, len(rows)),
	}

	// Collect valid company_ids for bulk lookup; track row ordering so we can
	// attach existing_id back without N+1.
	companyIDs := make([]string, 0, len(rows))
	rowIdx := map[string][]int{} // company_id -> list of row numbers (duplicates-within-file)
	for i, r := range rows {
		cid := strings.TrimSpace(r.CompanyID)
		if cid == "" || strings.TrimSpace(r.CompanyName) == "" {
			preview.Rows = append(preview.Rows, ImportPreviewRow{
				Row:         i + 2, // +2: 1-based + header row
				Status:      "invalid",
				CompanyID:   r.CompanyID,
				CompanyName: r.CompanyName,
				Error:       "company_id and company_name are required",
			})
			preview.Invalid++
			continue
		}
		companyIDs = append(companyIDs, cid)
		rowIdx[cid] = append(rowIdx[cid], i)
	}

	existing, err := u.repo.ExistingCompanyIDs(ctx, workspaceID, companyIDs)
	if err != nil {
		return nil, fmt.Errorf("preview import lookup: %w", err)
	}

	for i, r := range rows {
		cid := strings.TrimSpace(r.CompanyID)
		if cid == "" || strings.TrimSpace(r.CompanyName) == "" {
			continue // already recorded as invalid above
		}
		existingID, isDup := existing[cid]
		status := "new"
		if isDup {
			status = "duplicate"
			preview.Duplicates++
		} else {
			preview.New++
		}
		preview.Rows = append(preview.Rows, ImportPreviewRow{
			Row:         i + 2,
			Status:      status,
			CompanyID:   cid,
			CompanyName: r.CompanyName,
			ExistingID:  existingID,
		})
	}

	return preview, nil
}
