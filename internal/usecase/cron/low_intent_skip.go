package cron

import (
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// LowIntentSkippedTriggers is the set of BD sequence triggers that should be
// skipped when the prospect shows low buying intent. See features/06-workflow-
// engine/05-cron-engine §"Low-intent BD sequence shortening" — D12, D14, and
// D21 are courtesy follow-ups that have poor conversion on cold prospects; we
// skip them to avoid spam and free the BD up for higher-intent work.
var LowIntentSkippedTriggers = map[string]bool{
	"BD_D12_PATIENCE_NUDGE":    true,
	"BD_D14_FINAL_CHECKIN":     true,
	"BD_D21_LAST_ATTEMPT":      true,
}

// IsLowIntentSkip returns true when the given rule should be skipped for the
// given master_data row because of low-intent signals. Caller decides whether
// to actually skip (this function is a pure predicate).
//
// "Low-intent" is read from the standard BANTS fields populated by Claude
// extraction or manual BD input:
//
//   custom_fields.bants_classification = "cold"
//   custom_fields.buying_intent        = "low"
//
// Either signal is sufficient. Missing fields → not skipped (conservative).
func IsLowIntentSkip(rule entity.AutomationRule, md entity.MasterData) bool {
	if !LowIntentSkippedTriggers[rule.TriggerID] {
		return false
	}
	if md.CustomFields == nil {
		return false
	}
	if v, ok := md.CustomFields["bants_classification"].(string); ok && strings.EqualFold(v, "cold") {
		return true
	}
	if v, ok := md.CustomFields["buying_intent"].(string); ok && strings.EqualFold(v, "low") {
		return true
	}
	return false
}
