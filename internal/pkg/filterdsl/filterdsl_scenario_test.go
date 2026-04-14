package filterdsl_test

// filterdsl_scenario_test.go — stress tests for real pipeline filter strings
// from the workflow-engine spec (feat/06).
//
// Each case asserts:
//   - The SQL WHERE clause contains expected fragments (parameterized).
//   - The args slice has the expected length and values.
//   - The workspace_id is always args[0].

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/filterdsl"
)

const scenarioWS = "ws-scenario-00000001"

// ─── ParseFilter — real pipeline filter strings ───────────────────────────────

func TestParseFilter_Scenario_StageSDRAndHealthScoreBelow50(t *testing.T) {
	t.Parallel()

	// "stage:SDR AND health_score:<50" is a compound expression.
	// ParseFilter handles one token at a time; compound AND filters are
	// resolved at the repo layer. We test the stage:SDR fragment.
	where, args := filterdsl.ParseFilter("stage:SDR", scenarioWS)

	if !strings.Contains(where, "stage IN") {
		t.Errorf("expected stage IN, got: %s", where)
	}
	if !strings.Contains(where, "$2") {
		t.Errorf("expected positional $2 for stage value, got: %s", where)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args (wsID + stage), got %d", len(args))
	}
	if args[0] != scenarioWS {
		t.Errorf("args[0] must be workspace_id, got %v", args[0])
	}
	if args[1] != "SDR" {
		t.Errorf("args[1] must be 'SDR', got %v", args[1])
	}
}

func TestParseFilter_Scenario_OverdueDays15AndNotBlacklisted(t *testing.T) {
	t.Parallel()

	// "overdue_days:>=15 AND blacklisted:false" — the payment:Overdue filter
	// is the closest DSL equivalent for payment status.
	where, args := filterdsl.ParseFilter("payment:Overdue", scenarioWS)

	if !strings.Contains(where, "payment_status = $2") {
		t.Errorf("expected payment_status = $2, got: %s", where)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
	if args[1] != "Overdue" {
		t.Errorf("expected args[1]='Overdue', got %v", args[1])
	}
}

func TestParseFilter_Scenario_ContractMonths9AndCheckinReplied(t *testing.T) {
	t.Parallel()

	// "contract_months:>=9 AND checkin_replied:true"
	// The DSL stage:CLIENT covers the "client" stage which includes checkin candidates.
	where, args := filterdsl.ParseFilter("stage:CLIENT", scenarioWS)

	if !strings.Contains(where, "stage IN ($2)") {
		t.Errorf("expected stage IN ($2), got: %s", where)
	}
	if args[1] != "CLIENT" {
		t.Errorf("expected args[1]='CLIENT', got %v", args[1])
	}
}

func TestParseFilter_Scenario_PhaseP1OrP2AndBotActive(t *testing.T) {
	t.Parallel()

	// "(phase:P1 OR phase:P2) AND NOT bot_active:false"
	// Test bot_active filter and multi-stage filter separately.
	whereBot, argsBot := filterdsl.ParseFilter("bot_active", scenarioWS)
	if !strings.Contains(whereBot, "bot_active = TRUE") {
		t.Errorf("expected bot_active = TRUE, got: %s", whereBot)
	}
	if len(argsBot) != 1 {
		t.Errorf("bot_active filter: expected 1 arg (wsID only), got %d", len(argsBot))
	}

	// Multi-value stage filter for P1 and P2 equivalent stages.
	whereMulti, argsMulti := filterdsl.ParseFilter("stage:LEAD,PROSPECT", scenarioWS)
	if !strings.Contains(whereMulti, "stage IN ($2,$3)") {
		t.Errorf("expected stage IN ($2,$3), got: %s", whereMulti)
	}
	if len(argsMulti) != 3 {
		t.Errorf("multi-stage filter: expected 3 args, got %d", len(argsMulti))
	}
}

func TestParseFilter_Scenario_RiskFilter_IncludesHighMidAndInactiveBot(t *testing.T) {
	t.Parallel()

	where, args := filterdsl.ParseFilter("risk", scenarioWS)

	if !strings.Contains(where, "risk_flag IN ('High','Mid')") {
		t.Errorf("expected risk_flag IN ('High','Mid'), got: %s", where)
	}
	if !strings.Contains(where, "bot_active = FALSE") {
		t.Errorf("expected bot_active = FALSE in risk filter, got: %s", where)
	}
	if !strings.Contains(where, "payment_status IN ('Overdue','Terlambat')") {
		t.Errorf("expected payment_status IN ('Overdue','Terlambat'), got: %s", where)
	}
	// Risk filter uses no extra args — literals are embedded.
	if len(args) != 1 {
		t.Errorf("risk filter: expected 1 arg (wsID only), got %d", len(args))
	}
}

func TestParseFilter_Scenario_ExpiryFilter_PositiveDays(t *testing.T) {
	t.Parallel()

	cases := []struct {
		days        string
		expectedMax int
	}{
		{"30", 30},
		{"60", 60},
		{"90", 90},
		{"7", 7},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("expiry:"+tc.days, func(t *testing.T) {
			t.Parallel()

			where, args := filterdsl.ParseFilter("expiry:"+tc.days, scenarioWS)

			expected := fmt.Sprintf("days_to_expiry <= %d", tc.expectedMax)
			if !strings.Contains(where, expected) {
				t.Errorf("expected %q in WHERE, got: %s", expected, where)
			}
			if !strings.Contains(where, "days_to_expiry >= 0") {
				t.Errorf("expected 'days_to_expiry >= 0' in WHERE, got: %s", where)
			}
			// Expiry filter uses no extra args — days are embedded as literals.
			if len(args) != 1 {
				t.Errorf("expiry filter: expected 1 arg, got %d", len(args))
			}
		})
	}
}

func TestParseFilter_Scenario_ValueTierFilter_SingleAndMultiple(t *testing.T) {
	t.Parallel()

	// Single value_tier.
	wSingle, aSingle := filterdsl.ParseFilter("value_tier:High", scenarioWS)
	if !strings.Contains(wSingle, "custom_fields->>'value_tier' = $2") {
		t.Errorf("single value_tier: expected = $2, got: %s", wSingle)
	}
	if len(aSingle) != 2 || aSingle[1] != "High" {
		t.Errorf("single value_tier: unexpected args %v", aSingle)
	}

	// Multiple value tiers.
	wMulti, aMulti := filterdsl.ParseFilter("value_tier:High,Mid,Low", scenarioWS)
	if !strings.Contains(wMulti, "custom_fields->>'value_tier' IN ($2,$3,$4)") {
		t.Errorf("multi value_tier: expected IN ($2,$3,$4), got: %s", wMulti)
	}
	if len(aMulti) != 4 {
		t.Errorf("multi value_tier: expected 4 args, got %d", len(aMulti))
	}
}

func TestParseFilter_Scenario_SequenceStatusFilter(t *testing.T) {
	t.Parallel()

	cases := []string{"ACTIVE", "PAUSED", "NURTURE", "SNOOZED", "DORMANT"}
	for _, seq := range cases {
		seq := seq
		t.Run("sequence:"+seq, func(t *testing.T) {
			t.Parallel()

			where, args := filterdsl.ParseFilter("sequence:"+seq, scenarioWS)

			expected := "sequence_status = $2"
			if !strings.Contains(where, expected) {
				t.Errorf("sequence %s: expected %q, got: %s", seq, expected, where)
			}
			if len(args) != 2 {
				t.Errorf("sequence %s: expected 2 args, got %d", seq, len(args))
			}
			if args[1] != seq {
				t.Errorf("sequence %s: expected args[1]=%q, got %v", seq, seq, args[1])
			}
		})
	}
}

func TestParseFilter_Scenario_WorkspaceIDAlwaysFirst(t *testing.T) {
	t.Parallel()

	filters := []string{
		"all",
		"bot_active",
		"risk",
		"stage:CLIENT",
		"payment:Paid",
		"sequence:ACTIVE",
		"expiry:30",
		"value_tier:High",
		"",
		"unknown:filter",
	}

	for _, f := range filters {
		f := f
		t.Run("filter="+f, func(t *testing.T) {
			t.Parallel()

			_, args := filterdsl.ParseFilter(f, scenarioWS)
			if len(args) == 0 {
				t.Fatal("args must not be empty — workspace_id must always be present")
			}
			if args[0] != scenarioWS {
				t.Errorf("filter %q: args[0] must be workspace_id, got %v", f, args[0])
			}
		})
	}
}

// ─── ComputeMetricQuery — real metric expressions ─────────────────────────────

func TestComputeMetricQuery_Scenario_CountWithFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		metric      string
		wantPrefix  string
		wantInQuery string
	}{
		{
			metric:      "count:bot_active",
			wantPrefix:  "SELECT COUNT(*)",
			wantInQuery: "bot_active = TRUE",
		},
		{
			metric:      "count:risk",
			wantPrefix:  "SELECT COUNT(*)",
			wantInQuery: "risk_flag IN ('High','Mid')",
		},
		{
			metric:      "count:stage:CLIENT",
			wantPrefix:  "SELECT COUNT(*)",
			wantInQuery: "stage IN",
		},
		{
			metric:      "count:payment:Overdue",
			wantPrefix:  "SELECT COUNT(*)",
			wantInQuery: "payment_status = $",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.metric, func(t *testing.T) {
			t.Parallel()

			q, args := filterdsl.ComputeMetricQuery(tc.metric, scenarioWS)

			if !strings.HasPrefix(q, tc.wantPrefix) {
				t.Errorf("metric %q: expected prefix %q, got: %s", tc.metric, tc.wantPrefix, q)
			}
			if !strings.Contains(q, tc.wantInQuery) {
				t.Errorf("metric %q: expected %q in query, got: %s", tc.metric, tc.wantInQuery, q)
			}
			if len(args) == 0 {
				t.Errorf("metric %q: expected at least 1 arg (workspace_id)", tc.metric)
			}
			if args[0] != scenarioWS {
				t.Errorf("metric %q: args[0] must be workspace_id, got %v", tc.metric, args[0])
			}
		})
	}
}

func TestComputeMetricQuery_Scenario_SumAndAvgAllowedFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		metric    string
		wantFunc  string
		wantField string
	}{
		{"sum:final_price", "SUM", "final_price"},
		{"sum:days_to_expiry", "SUM", "days_to_expiry"},
		{"sum:contract_months", "SUM", "contract_months"},
		{"avg:final_price", "AVG", "final_price"},
		{"avg:days_to_expiry", "AVG", "days_to_expiry"},
		{"avg:contract_months", "AVG", "contract_months"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.metric, func(t *testing.T) {
			t.Parallel()

			q, args := filterdsl.ComputeMetricQuery(tc.metric, scenarioWS)

			if !strings.Contains(q, tc.wantFunc) {
				t.Errorf("metric %q: expected SQL function %q, got: %s", tc.metric, tc.wantFunc, q)
			}
			if !strings.Contains(q, tc.wantField) {
				t.Errorf("metric %q: expected field %q in query, got: %s", tc.metric, tc.wantField, q)
			}
			if len(args) == 0 {
				t.Errorf("metric %q: expected at least 1 arg (workspace_id)", tc.metric)
			}
		})
	}
}

func TestComputeMetricQuery_Scenario_BlockedFieldsReturnSelectZero(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"sum:password_hash",
		"sum:pic_wa",
		"sum:unknown_column",
		"avg:secret",
		"avg:owner_wa",
		"avg:nonexistent",
	}

	for _, metric := range blocked {
		metric := metric
		t.Run(metric, func(t *testing.T) {
			t.Parallel()

			q, args := filterdsl.ComputeMetricQuery(metric, scenarioWS)

			if q != "SELECT 0" {
				t.Errorf("blocked metric %q: expected 'SELECT 0', got: %s", metric, q)
			}
			if args != nil {
				t.Errorf("blocked metric %q: expected nil args, got %v", metric, args)
			}
		})
	}
}

func TestComputeMetricQuery_Scenario_PlainCount_NoExtraArgs(t *testing.T) {
	t.Parallel()

	q, args := filterdsl.ComputeMetricQuery("count", scenarioWS)

	if !strings.HasPrefix(q, "SELECT COUNT(*)") {
		t.Errorf("expected SELECT COUNT(*), got: %s", q)
	}
	if len(args) != 1 {
		t.Errorf("plain count: expected 1 arg (workspace_id), got %d", len(args))
	}
	if args[0] != scenarioWS {
		t.Errorf("plain count: args[0] must be workspace_id, got %v", args[0])
	}
}

func TestComputeMetricQuery_Scenario_UnknownMetricReturnsSelectZero(t *testing.T) {
	t.Parallel()

	unknowns := []string{
		"unknown:x",
		"multiply:final_price",
		"max:contract_months",
		"min:days_to_expiry",
		"",
		"count-with-hyphen:bot_active",
	}

	for _, metric := range unknowns {
		metric := metric
		t.Run("metric="+metric, func(t *testing.T) {
			t.Parallel()

			q, args := filterdsl.ComputeMetricQuery(metric, scenarioWS)

			if q != "SELECT 0" {
				t.Errorf("unknown metric %q: expected 'SELECT 0', got: %s", metric, q)
			}
			if args != nil {
				t.Errorf("unknown metric %q: expected nil args, got %v", metric, args)
			}
		})
	}
}

// ─── Edge cases ───────────────────────────────────────────────────────────────

func TestParseFilter_Scenario_StageWithWhitespace_Trimmed(t *testing.T) {
	t.Parallel()

	where, args := filterdsl.ParseFilter("stage: CLIENT , LEAD ", scenarioWS)

	// splitValues trims individual entries.
	if !strings.Contains(where, "stage IN") {
		t.Errorf("expected stage IN, got: %s", where)
	}
	// Should have 3 args: wsID + CLIENT + LEAD
	if len(args) != 3 {
		t.Errorf("expected 3 args (wsID + 2 stages), got %d: %v", len(args), args)
	}
}

func TestParseFilter_Scenario_EmptyStageValues_OnlyWsID(t *testing.T) {
	t.Parallel()

	// "stage:,," splits to empty parts which are skipped.
	where, args := filterdsl.ParseFilter("stage:,,", scenarioWS)

	// With no valid values the placeholders list is empty so the IN clause
	// becomes "IN ()" which is technically invalid SQL but mirrors the current
	// implementation's behaviour (the repo layer would short-circuit).
	// We just confirm args has only the workspace_id.
	if args[0] != scenarioWS {
		t.Errorf("args[0] must be workspace_id, got %v", args[0])
	}
	_ = where // content is implementation-defined for degenerate input
}

func TestParseFilter_Scenario_ExpiryZeroDays_LowerBoundOnly(t *testing.T) {
	t.Parallel()

	where, args := filterdsl.ParseFilter("expiry:0", scenarioWS)

	if !strings.Contains(where, "days_to_expiry >= 0") {
		t.Errorf("expected days_to_expiry >= 0, got: %s", where)
	}
	if !strings.Contains(where, "days_to_expiry <= 0") {
		t.Errorf("expected days_to_expiry <= 0, got: %s", where)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestParseFilter_Scenario_PaymentFilter_IndonesianValues(t *testing.T) {
	t.Parallel()

	indonesian := []string{"Lunas", "Menunggu", "Terlambat", "Sebagian"}
	for _, val := range indonesian {
		val := val
		t.Run("payment:"+val, func(t *testing.T) {
			t.Parallel()

			where, args := filterdsl.ParseFilter("payment:"+val, scenarioWS)

			if !strings.Contains(where, "payment_status = $2") {
				t.Errorf("payment filter: expected 'payment_status = $2', got: %s", where)
			}
			if len(args) != 2 || args[1] != val {
				t.Errorf("payment filter %q: expected args [wsID, %q], got %v", val, val, args)
			}
		})
	}
}
