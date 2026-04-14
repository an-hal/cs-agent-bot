package filterdsl_test

import (
	"strings"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/filterdsl"
)

const testWS = "00000000-0000-0000-0000-000000000001"

func TestParseFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		filter     string
		wantSuffix string // fragment that must appear in the WHERE clause
		wantArgs   int    // expected len(args)
	}{
		{
			name:       "empty is all",
			filter:     "",
			wantSuffix: "workspace_id = $1",
			wantArgs:   1,
		},
		{
			name:       "all",
			filter:     "all",
			wantSuffix: "workspace_id = $1",
			wantArgs:   1,
		},
		{
			name:       "bot_active",
			filter:     "bot_active",
			wantSuffix: "bot_active = TRUE",
			wantArgs:   1,
		},
		{
			name:       "risk",
			filter:     "risk",
			wantSuffix: "risk_flag IN ('High','Mid')",
			wantArgs:   1,
		},
		{
			name:       "stage single",
			filter:     "stage:LEAD",
			wantSuffix: "stage IN ($2)",
			wantArgs:   2,
		},
		{
			name:       "stage multi",
			filter:     "stage:LEAD,DORMANT",
			wantSuffix: "stage IN ($2,$3)",
			wantArgs:   3,
		},
		{
			name:       "value_tier single",
			filter:     "value_tier:High",
			wantSuffix: "custom_fields->>'value_tier' = $2",
			wantArgs:   2,
		},
		{
			name:       "value_tier multi",
			filter:     "value_tier:High,Mid",
			wantSuffix: "custom_fields->>'value_tier' IN ($2,$3)",
			wantArgs:   3,
		},
		{
			name:       "payment",
			filter:     "payment:Menunggu",
			wantSuffix: "payment_status = $2",
			wantArgs:   2,
		},
		{
			name:       "expiry",
			filter:     "expiry:30",
			wantSuffix: "days_to_expiry <= 30",
			wantArgs:   1,
		},
		{
			name:       "sequence",
			filter:     "sequence:ACTIVE",
			wantSuffix: "sequence_status = $2",
			wantArgs:   2,
		},
		{
			name:       "unknown defaults to all",
			filter:     "unknown_filter:xyz",
			wantSuffix: "workspace_id = $1",
			wantArgs:   1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			where, args := filterdsl.ParseFilter(tc.filter, testWS)
			if !strings.Contains(where, tc.wantSuffix) {
				t.Errorf("ParseFilter(%q): WHERE clause %q does not contain %q", tc.filter, where, tc.wantSuffix)
			}
			if len(args) != tc.wantArgs {
				t.Errorf("ParseFilter(%q): got %d args, want %d", tc.filter, len(args), tc.wantArgs)
			}
			// First arg must always be the workspace ID
			if len(args) > 0 && args[0] != testWS {
				t.Errorf("ParseFilter(%q): args[0] = %v, want %v", tc.filter, args[0], testWS)
			}
		})
	}
}

func TestComputeMetricQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		metric     string
		wantPrefix string
		nilArgs    bool // ComputeMetricQuery returns nil args for blocked/unknown fields
	}{
		{name: "count", metric: "count", wantPrefix: "SELECT COUNT(*)"},
		{name: "count:bot_active", metric: "count:bot_active", wantPrefix: "SELECT COUNT(*)"},
		{name: "count:risk", metric: "count:risk", wantPrefix: "SELECT COUNT(*)"},
		{name: "sum:final_price", metric: "sum:final_price", wantPrefix: "SELECT COALESCE(SUM(final_price)"},
		{name: "avg:days_to_expiry", metric: "avg:days_to_expiry", wantPrefix: "SELECT COALESCE(AVG(days_to_expiry)"},
		{name: "sum:blocked_field", metric: "sum:password_hash", wantPrefix: "SELECT 0", nilArgs: true},
		{name: "avg:blocked_field", metric: "avg:secret", wantPrefix: "SELECT 0", nilArgs: true},
		{name: "unknown", metric: "unknown:x", wantPrefix: "SELECT 0", nilArgs: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, args := filterdsl.ComputeMetricQuery(tc.metric, testWS)
			if !strings.HasPrefix(q, tc.wantPrefix) {
				t.Errorf("ComputeMetricQuery(%q): got %q, want prefix %q", tc.metric, q, tc.wantPrefix)
			}
			if tc.nilArgs && args != nil {
				t.Errorf("ComputeMetricQuery(%q): expected nil args, got %v", tc.metric, args)
			}
		})
	}
}
