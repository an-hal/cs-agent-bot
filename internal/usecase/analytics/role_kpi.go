package analytics

// Role identifiers accepted by the per-role KPI bundle.
const (
	RoleSDR   = "sdr"
	RoleBD    = "bd"
	RoleAE    = "ae"
	RoleAdmin = "admin"
)

// RoleKPIResult is the filtered KPI payload returned by /analytics/kpi/bundle.
// Each role sees a tuned subset per spec — SDR 4, BD 4, AE 6 metrics.
// Admin (or unknown role) gets the full KPI payload untouched.
type RoleKPIResult struct {
	Role    string         `json:"role"`
	Metrics map[string]any `json:"metrics"`
}

// kpiRoleLayout maps role → JSON keys to surface. Keys are matched against
// the JSON tags of entity.KPIData — if the entity doesn't expose a given
// field yet, the output silently drops it (forward-compatible).
var kpiRoleLayout = map[string][]string{
	RoleSDR: {
		// SDR-owned metrics
		"leads_this_month",
		"qualified_rate",
		"avg_response_time_hours",
		"active_prospects",
	},
	RoleBD: {
		// BD-owned metrics
		"prospects_in_pipeline",
		"win_rate",
		"avg_deal_cycle_days",
		"closed_won_this_month",
	},
	RoleAE: {
		// AE-owned metrics — full client lifecycle visibility
		"active_clients",
		"renewal_rate",
		"churn_rate",
		"mrr",
		"expansion_rate",
		"overdue_invoices_count",
	},
}

// BuildRoleKPIFromJSON projects a pre-marshaled KPI map down to the role's
// subset. Caller is expected to json.Marshal+Unmarshal the KPIData struct
// before calling so we work with JSON tag keys (not Go field names).
func BuildRoleKPIFromJSON(role string, raw map[string]any) RoleKPIResult {
	out := RoleKPIResult{Role: role, Metrics: map[string]any{}}
	if raw == nil {
		return out
	}
	if role == "" || role == RoleAdmin {
		out.Role = RoleAdmin
		out.Metrics = raw
		return out
	}
	keys, ok := kpiRoleLayout[role]
	if !ok {
		out.Role = RoleAdmin
		out.Metrics = raw
		return out
	}
	for _, k := range keys {
		if v, present := raw[k]; present {
			out.Metrics[k] = v
		}
	}
	return out
}
