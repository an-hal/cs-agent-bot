// OneSchema-style import: schema discovery + header auto-detect.
//
// The legacy import path required the user's xlsx to use exact template
// header names. This file adds two endpoints that turn the import flow into
// a wizard: (1) Schema returns the target schema so FE can render a column
// picker; (2) Detect inspects an uploaded xlsx and proposes a source→target
// mapping by fuzzy-matching headers. Apply/Preview later honor that mapping
// instead of relying on exact header equality.

package master_data

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/xuri/excelize/v2"
)

// ImportFieldDef is one target column in the import schema.
type ImportFieldDef struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // text|number|date|boolean|enum|email|phone|currency
	Required    bool     `json:"required"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
	IsCustom    bool     `json:"is_custom"`
}

// ImportSchema is the target schema FE consumes to render the mapping wizard.
type ImportSchema struct {
	CoreFields   []ImportFieldDef `json:"core_fields"`
	CustomFields []ImportFieldDef `json:"custom_fields"`
}

// ImportDetectSheet is one tab of an inspected xlsx.
type ImportDetectSheet struct {
	Name       string     `json:"name"`
	Headers    []string   `json:"headers"`
	SampleRows [][]string `json:"sample_rows"`
	RowCount   int        `json:"row_count"`
}

// MappingSuggestion proposes one source→target pairing with a confidence score.
type MappingSuggestion struct {
	SourceColumn string  `json:"source_column"`
	TargetKey    string  `json:"target_key,omitempty"`
	Confidence   float64 `json:"confidence"` // 0.0–1.0; <0.5 = no suggestion
}

// ImportDetectResult is what Detect returns to FE.
type ImportDetectResult struct {
	Sheets             []ImportDetectSheet `json:"sheets"`
	RecommendedSheet   string              `json:"recommended_sheet"`
	SuggestedMapping   []MappingSuggestion `json:"suggested_mapping"`
	UnmappedTargetKeys []string            `json:"unmapped_target_keys"`
}

// CoreFieldDefinitions is the canonical list of core import target fields.
// Order is the visual order FE should render in the mapping UI. Keys here MUST
// match the keys the parser/Patch path uses on entity.MasterData.
//
// Required flag mirrors xlsximport.ParseClientSheetWithDefs's hard validations
// (company_id, company_name, pic_name, pic_wa, owner_name); other "required"
// fields are NOT NULL at the DB level and either have defaults or are derived.
func CoreFieldDefinitions() []ImportFieldDef {
	stages := []string{entity.StageLead, entity.StageProspect, entity.StageClient, entity.StageDormant}
	risk := []string{entity.RiskNone, entity.RiskLow, entity.RiskMid, entity.RiskHigh}
	seqStatus := []string{
		entity.SeqStatusActive, entity.SeqStatusPaused, entity.SeqStatusNurture,
		entity.SeqStatusNurturePool, entity.SeqStatusSnoozed, entity.SeqStatusDormant,
	}
	billing := []string{"monthly", "quarterly", "annual", "one_time", "perpetual"}
	yesno := []string{"Yes", "No"}

	valueTier := []string{"HIGH", "MID", "LOW"}
	return []ImportFieldDef{
		{Key: "company_id", Label: "Company ID", Type: "text", Required: true, Description: "Unique business key (max 20 chars)"},
		{Key: "company_name", Label: "Company Name", Type: "text", Required: true},
		{Key: "stage", Label: "Stage", Type: "enum", Options: stages, Description: "Default: LEAD"},
		{Key: "industry", Label: "Industry", Type: "text", Description: "Retail / F&B / Manufaktur / Jasa / etc."},
		{Key: "value_tier", Label: "Value Tier", Type: "enum", Options: valueTier, Description: "ACV-based segmentation"},
		{Key: "pic_name", Label: "PIC Name", Type: "text", Required: true},
		{Key: "pic_nickname", Label: "PIC Nickname", Type: "text"},
		{Key: "pic_role", Label: "PIC Role", Type: "text"},
		{Key: "pic_wa", Label: "PIC WA", Type: "phone", Required: true, Description: "WhatsApp number; digits only"},
		{Key: "pic_email", Label: "PIC Email", Type: "email"},
		{Key: "owner_name", Label: "Owner Name", Type: "text", Required: true, Description: "Internal CS owner"},
		{Key: "owner_wa", Label: "Owner WA", Type: "phone"},
		{Key: "owner_telegram_id", Label: "Owner Telegram ID", Type: "text"},
		{Key: "bot_active", Label: "Bot Active", Type: "boolean", Options: yesno, Description: "Default: Yes"},
		{Key: "blacklisted", Label: "Blacklisted", Type: "boolean", Options: yesno, Description: "Default: No"},
		{Key: "sequence_status", Label: "Sequence Status", Type: "enum", Options: seqStatus, Description: "Default: ACTIVE"},
		{Key: "snooze_until", Label: "Snooze Until", Type: "date"},
		{Key: "snooze_reason", Label: "Snooze Reason", Type: "text"},
		{Key: "risk_flag", Label: "Risk Flag", Type: "enum", Options: risk, Description: "Default: None"},
		{Key: "contract_start", Label: "Contract Start", Type: "date"},
		{Key: "contract_end", Label: "Contract End", Type: "date"},
		{Key: "contract_months", Label: "Contract Months", Type: "number"},
		{Key: "payment_status", Label: "Payment Status", Type: "text", Description: "Default: Pending"},
		{Key: "payment_terms", Label: "Payment Terms", Type: "text"},
		{Key: "final_price", Label: "Final Price", Type: "currency"},
		{Key: "last_payment_date", Label: "Last Payment Date", Type: "date"},
		{Key: "billing_period", Label: "Billing Period", Type: "enum", Options: billing, Description: "Default: monthly"},
		{Key: "quantity", Label: "Quantity", Type: "number"},
		{Key: "unit_price", Label: "Unit Price", Type: "currency"},
		{Key: "currency", Label: "Currency", Type: "text", Description: "ISO 4217; default IDR"},
		{Key: "last_interaction_date", Label: "Last Interaction Date", Type: "date"},
		{Key: "notes", Label: "Notes", Type: "text"},
	}
}

// Schema returns the target import schema for a workspace: core fields + the
// workspace's custom field definitions, mapped into the same shape so FE can
// render a single mapping picker.
func (u *usecase) Schema(ctx context.Context, workspaceID string) (*ImportSchema, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	defs, err := u.cfdRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("load custom field defs: %w", err)
	}
	out := &ImportSchema{
		CoreFields:   CoreFieldDefinitions(),
		CustomFields: make([]ImportFieldDef, 0, len(defs)),
	}
	for _, d := range defs {
		f := ImportFieldDef{
			Key:         d.FieldKey,
			Label:       d.FieldLabel,
			Type:        d.FieldType,
			Required:    d.IsRequired,
			Description: d.Description,
			Options:     d.SelectOptions(),
			IsCustom:    true,
		}
		out.CustomFields = append(out.CustomFields, f)
	}
	return out, nil
}

// Detect inspects an uploaded xlsx and proposes a source→target column mapping.
// Picks the largest sheet by row count as the recommended one and runs fuzzy
// matching on its headers. Sample rows (first 3) are included so FE can show
// a live preview while the user adjusts the mapping.
func (u *usecase) Detect(ctx context.Context, workspaceID string, r io.Reader) (*ImportDetectResult, error) {
	if workspaceID == "" {
		return nil, apperror.ValidationError("workspace_id required")
	}
	schema, err := u.Schema(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, apperror.ValidationError("invalid xlsx: " + err.Error())
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, apperror.ValidationError("xlsx contains no sheets")
	}

	out := &ImportDetectResult{
		Sheets: make([]ImportDetectSheet, 0, len(sheets)),
	}
	bestSheet, bestRowCount := "", -1
	for _, s := range sheets {
		rows, err := f.GetRows(s)
		if err != nil || len(rows) == 0 {
			continue
		}
		headers := rows[0]
		dataRowCount := len(rows) - 1
		samples := make([][]string, 0, 3)
		for i := 1; i < len(rows) && len(samples) < 3; i++ {
			samples = append(samples, rows[i])
		}
		out.Sheets = append(out.Sheets, ImportDetectSheet{
			Name:       s,
			Headers:    headers,
			SampleRows: samples,
			RowCount:   dataRowCount,
		})
		if dataRowCount > bestRowCount {
			bestSheet = s
			bestRowCount = dataRowCount
		}
	}
	if bestSheet == "" {
		return nil, apperror.ValidationError("no sheet has data rows")
	}
	out.RecommendedSheet = bestSheet

	// Build the target field index — both core and custom — for matching.
	allTargets := make([]ImportFieldDef, 0, len(schema.CoreFields)+len(schema.CustomFields))
	allTargets = append(allTargets, schema.CoreFields...)
	allTargets = append(allTargets, schema.CustomFields...)

	// Match headers from the recommended sheet only (FE flips to others as
	// needed via Detect re-call or local re-map).
	var recHeaders []string
	for _, s := range out.Sheets {
		if s.Name == out.RecommendedSheet {
			recHeaders = s.Headers
			break
		}
	}

	out.SuggestedMapping = suggestMappings(recHeaders, allTargets)
	out.UnmappedTargetKeys = collectUnmappedTargets(out.SuggestedMapping, allTargets)
	return out, nil
}

// suggestMappings runs a fuzzy match for every source header against every
// target field's key/label. Returns one suggestion per source header — empty
// target_key when confidence < 0.5 (no match).
func suggestMappings(sources []string, targets []ImportFieldDef) []MappingSuggestion {
	out := make([]MappingSuggestion, 0, len(sources))
	used := make(map[string]bool, len(targets))
	for _, src := range sources {
		bestKey, bestScore := "", 0.0
		for _, tgt := range targets {
			if used[tgt.Key] {
				continue
			}
			score := matchScore(src, tgt)
			if score > bestScore {
				bestKey, bestScore = tgt.Key, score
			}
		}
		s := MappingSuggestion{SourceColumn: src, Confidence: bestScore}
		if bestScore >= 0.5 {
			s.TargetKey = bestKey
			used[bestKey] = true
		}
		out = append(out, s)
	}
	return out
}

// matchScore computes a 0–1 similarity between a source header and a target
// field. Combines: exact-match (1.0), normalized-equal (0.95), prefix/contains
// (0.7–0.85), and a token-overlap fallback for the rest.
func matchScore(src string, tgt ImportFieldDef) float64 {
	srcN := normalize(src)
	if srcN == "" {
		return 0
	}
	candidates := []string{normalize(tgt.Key), normalize(tgt.Label)}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if c == srcN {
			return 1.0
		}
	}
	best := 0.0
	for _, c := range candidates {
		if c == "" {
			continue
		}
		s := tokenOverlap(srcN, c)
		if s > best {
			best = s
		}
		if strings.Contains(srcN, c) || strings.Contains(c, srcN) {
			containment := 0.75
			if len(c) > 4 && len(srcN) > 4 {
				containment = 0.85
			}
			if containment > best {
				best = containment
			}
		}
	}
	return best
}

// normalize lowercases and reduces a column header to alphanumeric tokens
// joined by single underscores so "PIC WA" → "pic_wa", "Final Price" →
// "final_price", "Pic-Email" → "pic_email".
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevSep := true
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevSep = false
		default:
			if !prevSep {
				b.WriteRune('_')
				prevSep = true
			}
		}
	}
	out := b.String()
	return strings.Trim(out, "_")
}

// tokenOverlap returns the Jaccard-like ratio of shared tokens between two
// underscore-separated strings.
func tokenOverlap(a, b string) float64 {
	at := strings.Split(a, "_")
	bt := strings.Split(b, "_")
	if len(at) == 0 || len(bt) == 0 {
		return 0
	}
	bset := make(map[string]bool, len(bt))
	for _, t := range bt {
		if t != "" {
			bset[t] = true
		}
	}
	hits := 0
	for _, t := range at {
		if t != "" && bset[t] {
			hits++
		}
	}
	union := len(at) + len(bt) - hits
	if union == 0 {
		return 0
	}
	return float64(hits) / float64(union)
}

func collectUnmappedTargets(suggestions []MappingSuggestion, targets []ImportFieldDef) []string {
	mapped := make(map[string]bool, len(suggestions))
	for _, s := range suggestions {
		if s.TargetKey != "" {
			mapped[s.TargetKey] = true
		}
	}
	out := make([]string, 0)
	for _, t := range targets {
		if !mapped[t.Key] {
			out = append(out, t.Key)
		}
	}
	return out
}
