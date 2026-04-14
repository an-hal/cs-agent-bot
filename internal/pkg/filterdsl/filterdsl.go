// Package filterdsl implements the filter and metric DSL used by pipeline tabs
// and stat cards (spec: 00-shared/01-filter-dsl.md).
//
// Filter DSL syntax:
//
//	all                     — no additional filter
//	bot_active              — bot_active = TRUE
//	risk                    — risk_flag IN ('High','Mid') OR bot_active = FALSE OR payment_status IN ('Overdue','Terlambat')
//	stage:{v1},{v2}         — stage IN (...)
//	value_tier:{v1},{v2}    — custom_fields->>'value_tier' IN (...)
//	sequence:{value}        — sequence_status = '{value}'
//	payment:{value}         — payment_status = '{value}'
//	expiry:{days}           — days_to_expiry BETWEEN 0 AND {days}
//
// Metric DSL syntax:
//
//	count                   — COUNT(*)
//	count:{filter}          — COUNT(*) WHERE {filter applied}
//	sum:{field}             — SUM({field})
//	avg:{field}             — AVG({field})
package filterdsl

import (
	"fmt"
	"strings"
)

// FilterAll is the DSL value representing no additional filter beyond workspace_id.
const FilterAll = "all"

// allowedSumFields is the set of numeric master_data fields permitted in
// sum: metrics. This prevents arbitrary SQL column injection.
var allowedSumFields = map[string]bool{
	"final_price":    true,
	"days_to_expiry": true,
	"contract_months": true,
}

// allowedAvgFields mirrors allowedSumFields for avg: metrics.
var allowedAvgFields = map[string]bool{
	"final_price":    true,
	"days_to_expiry": true,
	"contract_months": true,
}

// ParseFilter converts a filter DSL string into a SQL WHERE fragment and
// positional argument slice. The caller must supply the base workspace_id
// argument at position $1; additional arguments are appended starting at $2.
//
// Example:
//
//	where, args := ParseFilter("stage:LEAD,PROSPECT", wsID)
//	// where == "workspace_id = $1 AND stage IN ($2,$3)"
//	// args  == [wsID, "LEAD", "PROSPECT"]
func ParseFilter(filter string, wsID string) (string, []interface{}) {
	base := "workspace_id = $1"
	args := []interface{}{wsID}

	switch {
	case filter == "" || filter == "all":
		return base, args

	case filter == "bot_active":
		return base + " AND bot_active = TRUE", args

	case filter == "risk":
		return base + " AND (risk_flag IN ('High','Mid') OR bot_active = FALSE OR payment_status IN ('Overdue','Terlambat'))", args

	case strings.HasPrefix(filter, "stage:"):
		vals := splitValues(filter[len("stage:"):])
		placeholders, newArgs := buildPlaceholders(args, vals)
		return base + " AND stage IN (" + placeholders + ")", newArgs

	case strings.HasPrefix(filter, "value_tier:"):
		vals := splitValues(filter[len("value_tier:"):])
		if len(vals) == 1 {
			args = append(args, vals[0])
			return base + fmt.Sprintf(" AND custom_fields->>'value_tier' = $%d", len(args)), args
		}
		placeholders, newArgs := buildPlaceholders(args, vals)
		return base + " AND custom_fields->>'value_tier' IN (" + placeholders + ")", newArgs

	case strings.HasPrefix(filter, "payment:"):
		val := strings.TrimSpace(filter[len("payment:"):])
		args = append(args, val)
		return base + fmt.Sprintf(" AND payment_status = $%d", len(args)), args

	case strings.HasPrefix(filter, "expiry:"):
		raw := strings.TrimSpace(filter[len("expiry:"):])
		days := 0
		fmt.Sscanf(raw, "%d", &days) //nolint:errcheck
		return base + fmt.Sprintf(" AND days_to_expiry >= 0 AND days_to_expiry <= %d", days), args

	case strings.HasPrefix(filter, "sequence:"):
		val := strings.TrimSpace(filter[len("sequence:"):])
		args = append(args, val)
		return base + fmt.Sprintf(" AND sequence_status = $%d", len(args)), args

	default:
		return base, args
	}
}

// ComputeMetricQuery returns the SQL query string and arguments for a given
// metric DSL expression against the master_data table.
//
// The caller executes the query and reads a single scalar result.
func ComputeMetricQuery(metric string, wsID string) (string, []interface{}) {
	args := []interface{}{wsID}

	switch {
	case metric == "count":
		return "SELECT COUNT(*) FROM master_data WHERE workspace_id = $1", args

	case strings.HasPrefix(metric, "count:"):
		f := metric[len("count:"):]
		where, fArgs := ParseFilter(f, wsID)
		return "SELECT COUNT(*) FROM master_data WHERE " + where, fArgs

	case strings.HasPrefix(metric, "sum:"):
		field := strings.TrimSpace(metric[len("sum:"):])
		if !allowedSumFields[field] {
			return "SELECT 0", nil
		}
		return fmt.Sprintf("SELECT COALESCE(SUM(%s), 0) FROM master_data WHERE workspace_id = $1", field), args

	case strings.HasPrefix(metric, "avg:"):
		field := strings.TrimSpace(metric[len("avg:"):])
		if !allowedAvgFields[field] {
			return "SELECT 0", nil
		}
		return fmt.Sprintf("SELECT COALESCE(AVG(%s)::int, 0) FROM master_data WHERE workspace_id = $1", field), args

	default:
		return "SELECT 0", nil
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// splitValues splits a comma-separated DSL value list and trims whitespace.
func splitValues(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// buildPlaceholders appends values to args and returns positional SQL placeholders.
func buildPlaceholders(existing []interface{}, vals []string) (string, []interface{}) {
	args := make([]interface{}, len(existing), len(existing)+len(vals))
	copy(args, existing)
	placeholders := make([]string, len(vals))
	for i, v := range vals {
		args = append(args, v)
		placeholders[i] = fmt.Sprintf("$%d", len(args))
	}
	return strings.Join(placeholders, ","), args
}
