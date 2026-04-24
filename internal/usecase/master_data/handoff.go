package master_data

import (
	"context"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

// HandoffFieldMap defines the BD → AE discovery field propagation contract.
// When BD moves a prospect to AE (stage transition prospect→client), these
// custom_field keys travel from BD-owned fields to AE-owned fields so the AE
// starts with the full context.
//
// Keys = BD source field (custom_fields.*).
// Values = AE target field (also custom_fields.*). Same-key pass-through is
// a no-op — it's kept to make the spec list explicit and auditable.
//
// See features/03-master-data + features/06-workflow-engine §"Handoff data contract".
var HandoffFieldMap = map[string]string{
	// Company discovery
	"hc_size":                "ae_hc_size",
	"industry":               "ae_industry",
	"company_website":        "ae_company_website",
	"company_linkedin":       "ae_company_linkedin",
	"current_hris_tool":      "ae_current_hris",
	"current_pain_point":     "ae_pain_point",
	"competitor_mentioned":   "ae_competitor",
	// Decision-maker discovery
	"dm_name":                "ae_dm_name",
	"dm_role":                "ae_dm_role",
	"dm_email":               "ae_dm_email",
	"dm_wa":                  "ae_dm_wa",
	"dm_buying_style":        "ae_dm_buying_style",
	"secondary_dm_name":      "ae_secondary_dm",
	// BANTS (from Claude extraction or manual)
	"bants_budget":           "ae_bants_budget",
	"bants_authority":        "ae_bants_authority",
	"bants_need":             "ae_bants_need",
	"bants_timing":           "ae_bants_timing",
	"bants_sentiment":        "ae_bants_sentiment",
	"bants_score":            "ae_bants_score",
	"bants_classification":   "ae_bants_classification",
	"buying_intent":          "ae_buying_intent",
	// Contract + pricing context
	"initial_quote_amount":   "ae_initial_quote",
	"quote_shared_at":        "ae_quote_shared_at",
	"preferred_payment_term": "ae_preferred_payment_term",
	"discount_requested":     "ae_discount_requested",
	"pilot_interest":         "ae_pilot_interest",
	// Meeting history
	"last_meeting_date":      "ae_last_meeting_date",
	"last_meeting_summary":   "ae_last_meeting_summary",
	"last_fireflies_id":      "ae_last_fireflies_id",
	// BD notes + next step
	"bd_next_step":           "ae_bd_next_step",
	"bd_urgency_notes":       "ae_bd_urgency_notes",
	"expected_close_date":    "ae_expected_close_date",
}

// ApplyHandoffMapping walks the map and, for every BD-side key that has a
// non-empty value in `source`, writes to the corresponding AE-side key in
// `target`. Non-destructive: if the target key already has a value the
// existing one wins (AE may have already edited).
//
// Returns the list of keys that were populated so the caller can log an audit
// trail + expose it to the AE as "prefilled from BD".
func ApplyHandoffMapping(source, target map[string]any) []string {
	if source == nil || target == nil {
		return nil
	}
	var populated []string
	for bdKey, aeKey := range HandoffFieldMap {
		v, ok := source[bdKey]
		if !ok {
			continue
		}
		if isEmpty(v) {
			continue
		}
		if existing, has := target[aeKey]; has && !isEmpty(existing) {
			continue
		}
		target[aeKey] = v
		populated = append(populated, aeKey)
	}
	return populated
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x) == ""
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

// ApplyBDHandoffToClient is the public entry point called during a stage
// transition prospect→client (or lead→client for "fast" paths). It loads the
// client, copies BD fields to AE fields, and persists via MergeCustomFields.
// Writes a mutation-log row tagged MutationSourceHandoff.
func (u *usecase) ApplyBDHandoffToClient(ctx context.Context, workspaceID, clientID, actorEmail string) ([]string, error) {
	if workspaceID == "" || clientID == "" {
		return nil, apperror.ValidationError("workspace_id and client_id required")
	}
	m, err := u.repo.GetByID(ctx, workspaceID, clientID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, apperror.NotFound("master_data", clientID)
	}
	target := map[string]any{}
	for k, v := range m.CustomFields {
		target[k] = v
	}
	populated := ApplyHandoffMapping(m.CustomFields, target)
	if len(populated) == 0 {
		return nil, nil
	}
	// Apply only the newly populated keys (so we don't clobber). MergeCustomFields
	// already does partial merge, so we can pass the full `target` safely.
	onlyNew := map[string]any{}
	for _, k := range populated {
		onlyNew[k] = target[k]
	}
	if err := u.repo.MergeCustomFields(ctx, workspaceID, clientID, onlyNew); err != nil {
		return nil, err
	}
	_ = u.mutationRepo.Append(ctx, &entity.MasterDataMutation{
		WorkspaceID:   workspaceID,
		MasterDataID:  clientID,
		CompanyID:     m.CompanyID,
		CompanyName:   m.CompanyName,
		Action:        "bd_ae_handoff",
		Source:        entity.MutationSourceHandoff,
		ActorEmail:    actorEmail,
		ChangedFields: populated,
		NewValues:     onlyNew,
		Note:          "BD→AE discovery fields auto-populated on stage transition",
	})
	return populated, nil
}
