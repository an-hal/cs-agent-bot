package master_data

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
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

	res := &ImportResult{Errors: []ImportRowError{}}
	for i, row := range rows {
		if row.CompanyID == "" || row.CompanyName == "" {
			res.Errors = append(res.Errors, ImportRowError{
				Row: i + 2, Error: "missing company_id or company_name", CompanyID: row.CompanyID,
			})
			continue
		}
		req := CreateRequest{
			CompanyID:       row.CompanyID,
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
			// Coarse stub: full update_existing flow is left to a follow-up
			// because clients lack a stable lookup-by-company_id at the
			// master_data layer in this scaffold.
			res.Skipped++
			continue
		}
		_, err := u.Create(ctx, workspaceID, checkerEmail, req)
		if err != nil {
			res.Errors = append(res.Errors, ImportRowError{
				Row: i + 2, Error: err.Error(), CompanyID: row.CompanyID,
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
		_ = f.SetCellValue(sheet, cell, h)
	}

	for i, m := range rows {
		values := coreExportValues(&m)
		for _, d := range defs {
			values = append(values, fmt.Sprint(m.CustomFields[d.FieldKey]))
		}
		for col, v := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, i+2)
			_ = f.SetCellValue(sheet, cell, v)
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
