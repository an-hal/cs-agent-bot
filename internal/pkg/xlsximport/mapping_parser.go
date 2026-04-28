// OneSchema-style mapping-aware parser. The legacy ParseClientSheetWithDefs
// requires the source xlsx to use the canonical template headers verbatim;
// mapping_parser drops that constraint by accepting an explicit
// source-header → target-field-key map. Sheet name and header set are also
// caller-controlled, which lets the wizard import any spreadsheet without
// pre-formatting.
//
// Phase B: type-driven cell transforms. Each cell value is coerced according
// to the target field's type (date/bool/phone/email/currency/enum/number),
// with errors surfaced as CellError so FE can highlight bad cells.

package xlsximport

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/xuri/excelize/v2"
)

// MappingParseOptions tells the parser which sheet to read and how each
// source header maps to a target field key. Empty target keys (or sources
// absent from Mapping) are silently skipped.
type MappingParseOptions struct {
	SheetName string
	// Mapping: source header (verbatim from xlsx) → target field key.
	// Target keys must match either a CoreFieldDefinitions key or a custom
	// field key from CustomFieldDefinitionRepository.
	Mapping map[string]string
	// Overrides (Phase C inline-edit): row_num (1-based, where 1 = header) →
	// {target_key: corrected_raw_string}. When the parser fetches a cell, an
	// override for that (row, target) replaces the source column's value
	// before transforms run. Lets a wizard-style FE patch bad cells without
	// re-uploading the source file.
	Overrides map[int]map[string]string
}

// CellError is one cell-level validation failure. Replaces the row-only
// ParseError model so FE can highlight the offending column.
type CellError struct {
	Row          int    `json:"row"`
	Column       string `json:"column"` // source header
	TargetKey    string `json:"target_key,omitempty"`
	SourceValue  string `json:"source_value"`
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// coreFieldType is the parser's view of each core target field's type, used
// for picking the right Transform* function. Mirrors the metadata in
// master_data.CoreFieldDefinitions but kept here to avoid an import cycle
// (xlsximport → master_data → xlsximport).
var coreFieldType = map[string]string{
	"company_id":            "text",
	"company_name":          "text",
	"stage":                 "enum",
	"pic_name":              "text",
	"pic_nickname":          "text",
	"pic_role":              "text",
	"pic_wa":                "phone",
	"pic_email":             "email",
	"owner_name":            "text",
	"owner_wa":              "phone",
	"owner_telegram_id":     "text",
	"bot_active":            "boolean",
	"blacklisted":           "boolean",
	"sequence_status":       "enum",
	"snooze_until":          "date",
	"snooze_reason":         "text",
	"risk_flag":             "enum",
	"contract_start":        "date",
	"contract_end":          "date",
	"contract_months":       "number",
	"payment_status":        "text",
	"payment_terms":         "text",
	"final_price":           "currency",
	"last_payment_date":     "date",
	"billing_period":        "enum",
	"quantity":              "number",
	"unit_price":            "currency",
	"currency":              "text",
	"last_interaction_date": "date",
	"notes":                 "text",
}

// Allowed enum values, used for Transform inside the parser. Same constants
// the master_data package validates against — duplicated here to avoid the
// import cycle.
var (
	enumStage          = []string{"LEAD", "PROSPECT", "CLIENT", "DORMANT"}
	enumSequenceStatus = []string{"ACTIVE", "PAUSED", "NURTURE", "NURTURE_POOL", "SNOOZED", "DORMANT"}
	enumRiskFlag       = []string{"None", "Low", "Mid", "High"}
	enumBillingPeriod  = []string{"monthly", "quarterly", "annual", "one_time", "perpetual"}
)

func enumOptionsFor(targetKey string) []string {
	switch targetKey {
	case "stage":
		return enumStage
	case "sequence_status":
		return enumSequenceStatus
	case "risk_flag":
		return enumRiskFlag
	case "billing_period":
		return enumBillingPeriod
	}
	return nil
}

// ParseClientSheetWithMapping reads the named sheet and produces ClientImportRow
// values using the caller's mapping. Required-field validation runs against the
// target keys (company_id/company_name/pic_name/pic_wa/owner_name); type
// coercion runs per target type. Per-cell errors are returned alongside rows.
//
// Custom field keys in the mapping (anything matching a def's field_key) are
// extracted into ClientImportRow.CustomFields with type-aware conversion.
func ParseClientSheetWithMapping(
	r io.Reader,
	opts MappingParseOptions,
	defs []entity.CustomFieldDefinition,
) ([]ClientImportRow, []CellError, error) {
	if opts.SheetName == "" {
		opts.SheetName = sheetName
	}
	if len(opts.Mapping) == 0 {
		return nil, nil, fmt.Errorf("mapping is empty; nothing to parse")
	}

	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open xlsx: %w", err)
	}
	defer f.Close()

	rows, err := f.GetRows(opts.SheetName)
	if err != nil {
		return nil, nil, fmt.Errorf("sheet %q not found: %w", opts.SheetName, err)
	}
	if len(rows) < 2 {
		return nil, nil, nil
	}

	headers := rows[0]
	headerIdx := buildHeaderIndex(headers)

	// Build target-key → column-index by following the mapping.
	target := make(map[string]int, len(opts.Mapping))
	for src, tgt := range opts.Mapping {
		tgt = strings.TrimSpace(tgt)
		if tgt == "" {
			continue
		}
		if i, ok := headerIdx[strings.TrimSpace(src)]; ok {
			target[tgt] = i
		}
	}

	// Defs lookup so we know which target keys are custom fields + their type.
	defByKey := make(map[string]entity.CustomFieldDefinition, len(defs))
	for _, d := range defs {
		defByKey[d.FieldKey] = d
	}

	// Inverse map for cell-error reporting.
	srcByTarget := make(map[string]string, len(opts.Mapping))
	for src, tgt := range opts.Mapping {
		if tgt != "" {
			srcByTarget[tgt] = src
		}
	}

	var results []ClientImportRow
	var cellErrs []CellError

	for rowNum, row := range rows[1:] {
		lineNum := rowNum + 2 // 1-indexed; row 1 = header

		if isRowEmpty(row) {
			continue
		}

		ctx := &rowParseCtx{
			row:         row,
			lineNum:     lineNum,
			target:      target,
			srcByTarget: srcByTarget,
			overrides:   opts.Overrides[lineNum],
		}

		parsed := ClientImportRow{
			CompanyID:           ctx.text("company_id", 20, true),
			CompanyName:         ctx.text("company_name", 0, true),
			Stage:               ctx.enum("stage", enumStage),
			PICName:             ctx.text("pic_name", 0, true),
			PICNickname:         ctx.text("pic_nickname", 0, false),
			PICRole:             ctx.text("pic_role", 0, false),
			PICWA:               ctx.phone("pic_wa", true),
			PICEmail:            ctx.email("pic_email"),
			OwnerName:           ctx.text("owner_name", 0, true),
			OwnerWA:             ctx.phone("owner_wa", false),
			OwnerTelegramID:     ctx.text("owner_telegram_id", 0, false),
			ContractStart:       ctx.date("contract_start"),
			ContractEnd:         ctx.date("contract_end"),
			ContractMonths:      ctx.intval("contract_months"),
			PaymentTerms:        ctx.text("payment_terms", 0, false),
			PaymentStatus:       mapPaymentStatus(ctx.text("payment_status", 0, false)),
			FinalPrice:          ctx.currency("final_price"),
			LastPaymentDate:     ctx.datePtr("last_payment_date"),
			BillingPeriod:       ctx.enum("billing_period", enumBillingPeriod),
			Quantity:            ctx.intPtr("quantity"),
			UnitPrice:           ctx.currencyPtr("unit_price"),
			Currency:            ctx.text("currency", 0, false),
			SequenceStatus:      ctx.enum("sequence_status", enumSequenceStatus),
			SnoozeUntil:         ctx.datePtr("snooze_until"),
			SnoozeReason:        ctx.text("snooze_reason", 0, false),
			RiskFlag:            ctx.enum("risk_flag", enumRiskFlag),
			BotActive:           ctx.boolean("bot_active", true),
			Blacklisted:         ctx.boolean("blacklisted", false),
			LastInteractionDate: ctx.datePtr("last_interaction_date"),
			Notes:               ctx.text("notes", 0, false),
		}
		// Legacy Segment field — mirror RiskFlag for backward compat.
		parsed.Segment = mapRiskFlag(parsed.RiskFlag)

		// Required-field guard: if any required field was missing, emit
		// errors and drop the row.
		if len(ctx.missingReqs) > 0 {
			cellErrs = append(cellErrs, ctx.missingReqs...)
			continue
		}
		// Fatal cell errors (e.g., invalid date in a required date column)
		// drop the row; non-fatal errors are surfaced but the row still
		// imports with whatever we managed to parse.
		if ctx.fatal {
			cellErrs = append(cellErrs, ctx.cellErrs...)
			continue
		}
		cellErrs = append(cellErrs, ctx.cellErrs...)

		// Custom field extraction — defByKey-driven, routed through TransformValue.
		custom := map[string]any{}
		for tgt, i := range target {
			def, isCustom := defByKey[tgt]
			if !isCustom {
				continue
			}
			var raw string
			if v, ok := ctx.overrides[tgt]; ok {
				raw = strings.TrimSpace(v)
			} else if i < len(row) {
				raw = strings.TrimSpace(row[i])
			}
			if raw == "" {
				continue
			}
			val, terr := TransformValue(raw, def.FieldType, def.SelectOptions())
			if terr != nil {
				cellErrs = append(cellErrs, CellError{
					Row:          lineNum,
					Column:       srcByTarget[tgt],
					TargetKey:    tgt,
					SourceValue:  raw,
					ErrorCode:    terr.Code,
					ErrorMessage: terr.Message,
				})
				continue
			}
			if val != nil {
				custom[tgt] = val
			}
		}
		if len(custom) > 0 {
			parsed.CustomFields = custom
		}
		results = append(results, parsed)
	}
	return results, cellErrs, nil
}

// rowParseCtx is the per-row scratchpad: it owns the raw cells, the running
// list of cell errors, and convenience methods that route each target field
// through the right Transform* function.
type rowParseCtx struct {
	row         []string
	lineNum     int
	target      map[string]int
	srcByTarget map[string]string
	overrides   map[string]string // overrides[targetKey] for this row, if any
	cellErrs    []CellError
	missingReqs []CellError
	fatal       bool
}

func (c *rowParseCtx) raw(key string) string {
	if v, ok := c.overrides[key]; ok {
		return strings.TrimSpace(v)
	}
	i, ok := c.target[key]
	if !ok || i >= len(c.row) {
		return ""
	}
	return strings.TrimSpace(c.row[i])
}

func (c *rowParseCtx) addErr(key, raw string, terr *TransformError) {
	if terr == nil {
		return
	}
	c.cellErrs = append(c.cellErrs, CellError{
		Row:          c.lineNum,
		Column:       c.srcByTarget[key],
		TargetKey:    key,
		SourceValue:  raw,
		ErrorCode:    terr.Code,
		ErrorMessage: terr.Message,
	})
}

func (c *rowParseCtx) addRequired(key string) {
	c.missingReqs = append(c.missingReqs, CellError{
		Row:          c.lineNum,
		Column:       c.srcByTarget[key],
		TargetKey:    key,
		ErrorCode:    "required",
		ErrorMessage: key + " is required",
	})
}

func (c *rowParseCtx) text(key string, max int, required bool) string {
	raw := c.raw(key)
	if raw == "" {
		if required {
			c.addRequired(key)
		}
		return ""
	}
	v, terr := TransformText(raw, max)
	if terr != nil {
		c.addErr(key, raw, terr)
		c.fatal = true
		return v
	}
	return v
}

func (c *rowParseCtx) phone(key string, required bool) string {
	raw := c.raw(key)
	if raw == "" {
		if required {
			c.addRequired(key)
		}
		return ""
	}
	v, terr := TransformPhone(raw)
	if terr != nil {
		c.addErr(key, raw, terr)
		// Phone validation issues are non-fatal — keep the row, surface the warning.
		return v
	}
	return v
}

func (c *rowParseCtx) email(key string) string {
	raw := c.raw(key)
	if raw == "" {
		return ""
	}
	v, terr := TransformEmail(raw)
	if terr != nil {
		c.addErr(key, raw, terr)
		// Non-fatal; many imports legitimately have empty/loose email cells.
		return v
	}
	return v
}

func (c *rowParseCtx) date(key string) time.Time {
	raw := c.raw(key)
	if raw == "" {
		return time.Time{}
	}
	t, terr := TransformDate(raw)
	if terr != nil {
		c.addErr(key, raw, terr)
		c.fatal = true
		return time.Time{}
	}
	return t
}

func (c *rowParseCtx) datePtr(key string) *time.Time {
	raw := c.raw(key)
	if raw == "" {
		return nil
	}
	t, terr := TransformDate(raw)
	if terr != nil {
		c.addErr(key, raw, terr)
		// Non-fatal for nullable date columns.
		return nil
	}
	if t.IsZero() {
		return nil
	}
	return &t
}

func (c *rowParseCtx) intval(key string) int {
	raw := c.raw(key)
	if raw == "" {
		return 0
	}
	v, terr := TransformInt(raw)
	if terr != nil {
		c.addErr(key, raw, terr)
		return 0
	}
	return v
}

func (c *rowParseCtx) intPtr(key string) *int {
	raw := c.raw(key)
	if raw == "" {
		return nil
	}
	v, terr := TransformInt(raw)
	if terr != nil {
		c.addErr(key, raw, terr)
		return nil
	}
	return &v
}

func (c *rowParseCtx) currency(key string) float64 {
	raw := c.raw(key)
	if raw == "" {
		return 0
	}
	v, terr := TransformCurrency(raw)
	if terr != nil {
		c.addErr(key, raw, terr)
		return 0
	}
	return v
}

func (c *rowParseCtx) currencyPtr(key string) *float64 {
	raw := c.raw(key)
	if raw == "" {
		return nil
	}
	v, terr := TransformCurrency(raw)
	if terr != nil {
		c.addErr(key, raw, terr)
		return nil
	}
	return &v
}

func (c *rowParseCtx) boolean(key string, def bool) bool {
	raw := c.raw(key)
	v, terr := TransformBool(raw, def)
	if terr != nil {
		c.addErr(key, raw, terr)
		return def
	}
	return v
}

func (c *rowParseCtx) enum(key string, options []string) string {
	raw := c.raw(key)
	if raw == "" {
		return ""
	}
	v, terr := TransformEnum(raw, options)
	if terr != nil {
		c.addErr(key, raw, terr)
		// Non-fatal: the value is preserved, FE highlights, user can fix.
		return v
	}
	return v
}

func isRowEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
