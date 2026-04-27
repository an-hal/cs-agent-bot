// Package xlsximport parses client XLSX import files based on the standard import template.
package xlsximport

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/xuri/excelize/v2"
)

// ClientImportRow holds all parsed fields from one data row of the import template.
type ClientImportRow struct {
	// Core identity
	CompanyID   string
	CompanyName string

	// Contact
	PICName         string
	PICRole         string
	PICWA           string
	PICEmail        string
	OwnerName       string
	OwnerWA         string
	OwnerTelegramID string

	// Contract
	ContractStart  time.Time
	ContractEnd    time.Time
	ContractMonths int

	// Financials
	PaymentTerms    string
	FinalPrice      float64
	PaymentStatus   string
	LastPaymentDate *time.Time
	QuotationLink   string

	// Scores
	NPSScore   int
	UsageScore int

	// Lifecycle
	HCSize              string
	PlanType            string
	SequenceCS          string
	Renewed             bool
	BotActive           bool
	Blacklisted         bool
	CheckinReplied      bool
	CrossSellInterested bool
	CrossSellRejected   bool
	CrossSellResumeDate *time.Time
	LastInteractionDate *time.Time
	Segment             string // mapped from Risk Flag: High/Mid/Low
	Notes               string

	// Custom — populated when caller passes field definitions to
	// ParseClientSheetWithDefs. Header lookup is case-insensitive against both
	// FieldLabel and FieldKey. Empty cells are skipped.
	CustomFields map[string]any
}

// ParseError represents a non-fatal per-row parse error.
type ParseError struct {
	Row    int
	RefID  string
	Reason string
}

const sheetName = "Template Import"

// ParseClientSheet reads the "Template Import" sheet without custom-field
// extraction. Equivalent to ParseClientSheetWithDefs(r, nil).
func ParseClientSheet(r io.Reader) ([]ClientImportRow, []ParseError, error) {
	return ParseClientSheetWithDefs(r, nil)
}

// ParseClientSheetWithDefs reads the "Template Import" sheet and returns
// parsed rows plus any per-row errors. When defs is non-empty, additional
// columns whose header matches a definition's FieldLabel or FieldKey
// (case-insensitive) are extracted into ClientImportRow.CustomFields.
//
// Rows with missing required fields are skipped and recorded as ParseErrors.
func ParseClientSheetWithDefs(r io.Reader, defs []entity.CustomFieldDefinition) ([]ClientImportRow, []ParseError, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open xlsx: %w", err)
	}
	defer f.Close()

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, nil, fmt.Errorf("sheet %q not found: %w", sheetName, err)
	}
	if len(rows) < 2 {
		return nil, nil, nil
	}

	idx := buildHeaderIndex(rows[0])
	customColIdx := buildCustomFieldColumnIndex(rows[0], defs)

	var results []ClientImportRow
	var parseErrs []ParseError

	for rowNum, row := range rows[1:] {
		lineNum := rowNum + 2 // 1-indexed, row 1 = header

		get := func(header string) string {
			i, ok := idx[header]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}

		companyID := get("Company ID")
		companyName := get("Company Name")
		picWA := get("PIC WA")

		// skip completely empty rows
		if companyID == "" && companyName == "" {
			continue
		}

		var rowErrors []string
		if companyID == "" {
			rowErrors = append(rowErrors, "company_id is required")
		}
		if companyName == "" {
			rowErrors = append(rowErrors, "company_name is required")
		}
		if get("PIC Name") == "" {
			rowErrors = append(rowErrors, "pic_name is required")
		}
		if picWA == "" {
			rowErrors = append(rowErrors, "pic_wa is required")
		}
		if get("Owner Name") == "" {
			rowErrors = append(rowErrors, "owner_name is required")
		}

		contractStart, csErr := parseDate(get("Contract Start"))
		if csErr != nil {
			rowErrors = append(rowErrors, "invalid contract_start: "+csErr.Error())
		}
		contractEnd, ceErr := parseDate(get("Contract End"))
		if ceErr != nil {
			rowErrors = append(rowErrors, "invalid contract_end: "+ceErr.Error())
		}

		if len(rowErrors) > 0 {
			parseErrs = append(parseErrs, ParseError{
				Row:    lineNum,
				RefID:  companyID,
				Reason: strings.Join(rowErrors, "; "),
			})
			continue
		}

		contractMonths, _ := strconv.Atoi(get("Contract Months"))
		finalPrice, _ := strconv.ParseFloat(get("Final Price"), 64)
		npsScore, _ := strconv.Atoi(get("NPS Score"))
		usageScore, _ := strconv.Atoi(get("Usage Score"))

		parsedRow := ClientImportRow{
			CompanyID:           companyID,
			CompanyName:         companyName,
			PICName:             get("PIC Name"),
			PICRole:             get("PIC Role"),
			PICWA:               picWA,
			PICEmail:            get("PIC Email"),
			OwnerName:           get("Owner Name"),
			OwnerWA:             get("Owner WA"),
			OwnerTelegramID:     get("Owner Telegram ID"),
			ContractStart:       contractStart,
			ContractEnd:         contractEnd,
			ContractMonths:      contractMonths,
			HCSize:              get("HC Size"),
			PlanType:            get("Plan Type"),
			PaymentTerms:        get("Payment Terms"),
			FinalPrice:          finalPrice,
			PaymentStatus:       mapPaymentStatus(get("Payment Status")),
			QuotationLink:       get("Quotation Link"),
			NPSScore:            npsScore,
			UsageScore:          usageScore,
			SequenceCS:          mapSequenceCS(get("Stage"), get("Sequence CS")),
			Renewed:             parseBool(get("Renewed")),
			BotActive:           parseBoolDefault(get("Bot Active"), true),
			Blacklisted:         parseBool(get("Blacklisted")),
			CheckinReplied:      parseBool(get("Checkin Replied")),
			CrossSellInterested: parseBool(get("Cross Sell Interested")),
			CrossSellRejected:   parseBool(get("Cross Sell Rejected")),
			Segment:             mapRiskFlag(get("Risk Flag")),
			Notes:               get("Notes"),
			LastPaymentDate:     parseNullableDate(get("Last Payment Date")),
			LastInteractionDate: parseNullableDate(get("Last Interaction")),
			CrossSellResumeDate: parseNullableDate(get("Cross Sell Resume Date")),
		}
		if cf := extractCustomFields(row, customColIdx, defs); len(cf) > 0 {
			parsedRow.CustomFields = cf
		}
		results = append(results, parsedRow)
	}

	return results, parseErrs, nil
}

func buildHeaderIndex(headers []string) map[string]int {
	idx := make(map[string]int, len(headers))
	for i, h := range headers {
		idx[strings.TrimSpace(h)] = i
	}
	return idx
}

// buildCustomFieldColumnIndex returns a map of field_key → column index, by
// matching each definition's FieldLabel or FieldKey (case-insensitive) against
// the xlsx header row. Definitions without a matching column are absent from
// the result.
func buildCustomFieldColumnIndex(headers []string, defs []entity.CustomFieldDefinition) map[string]int {
	if len(defs) == 0 {
		return nil
	}
	lower := make(map[string]int, len(headers))
	for i, h := range headers {
		lower[strings.ToLower(strings.TrimSpace(h))] = i
	}
	idx := make(map[string]int, len(defs))
	for _, d := range defs {
		if i, ok := lower[strings.ToLower(d.FieldLabel)]; ok {
			idx[d.FieldKey] = i
			continue
		}
		if i, ok := lower[strings.ToLower(d.FieldKey)]; ok {
			idx[d.FieldKey] = i
		}
	}
	return idx
}

// extractCustomFields pulls custom-field values from a row using the column
// index built by buildCustomFieldColumnIndex. Empty cells are skipped. Values
// are converted by FieldType (number/boolean/date stay typed; everything else
// stays string). Caller (validation pipeline) re-checks against options.
func extractCustomFields(row []string, colIdx map[string]int, defs []entity.CustomFieldDefinition) map[string]any {
	if len(colIdx) == 0 {
		return nil
	}
	defByKey := make(map[string]entity.CustomFieldDefinition, len(defs))
	for _, d := range defs {
		defByKey[d.FieldKey] = d
	}

	out := make(map[string]any, len(colIdx))
	for key, i := range colIdx {
		if i >= len(row) {
			continue
		}
		raw := strings.TrimSpace(row[i])
		if raw == "" {
			continue
		}
		def := defByKey[key]
		switch def.FieldType {
		case entity.FieldTypeNumber:
			if n, err := strconv.ParseFloat(raw, 64); err == nil {
				out[key] = n
			} else {
				out[key] = raw // keep raw; validator will reject
			}
		case entity.FieldTypeBoolean:
			out[key] = parseBool(raw)
		case entity.FieldTypeDate:
			if t, err := time.Parse("2006-01-02", raw); err == nil {
				out[key] = t.Format("2006-01-02")
			} else {
				out[key] = raw
			}
		default:
			out[key] = raw
		}
	}
	return out
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("date is empty")
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected YYYY-MM-DD, got %q", s)
	}
	return t, nil
}

func parseNullableDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

func parseBool(s string) bool {
	return strings.EqualFold(s, "yes") || strings.EqualFold(s, "true") || s == "1"
}

func parseBoolDefault(s string, def bool) bool {
	if s == "" {
		return def
	}
	return parseBool(s)
}

// mapPaymentStatus normalises Indonesian status names to internal English constants.
func mapPaymentStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "lunas":
		return "Paid"
	case "menunggu":
		return "Pending"
	case "belum bayar", "terlambat":
		return "Overdue"
	case "paid":
		return "Paid"
	case "pending":
		return "Pending"
	case "overdue":
		return "Overdue"
	case "partial":
		return "Partial"
	default:
		return "Paid"
	}
}

// mapSequenceCS resolves sequence_cs from Stage (primary) or Sequence CS column (override).
func mapSequenceCS(stage, seqCS string) string {
	if seqCS != "" {
		return strings.ToUpper(seqCS)
	}
	switch strings.ToUpper(strings.TrimSpace(stage)) {
	case "ACTIVE", "SNOOZED":
		return strings.ToUpper(stage)
	}
	return "ACTIVE"
}

// mapRiskFlag converts High/Mid/Low risk flag to the Segment field value.
func mapRiskFlag(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high":
		return "High"
	case "mid":
		return "Mid"
	case "low":
		return "Low"
	}
	return ""
}
