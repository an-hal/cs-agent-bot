package collection

import (
	"strings"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

func TestParseFilterDSL(t *testing.T) {
	fields := []entity.CollectionField{
		{Key: "category", Type: entity.ColFieldEnum},
		{Key: "title", Type: entity.ColFieldText},
		{Key: "created_on", Type: entity.ColFieldDate},
	}

	tests := []struct {
		name       string
		filter     string
		startArg   int
		wantSQL    string
		wantArgs   []any
		wantErrSub string
	}{
		{
			name:     "empty filter returns empty",
			filter:   "",
			startArg: 1,
			wantSQL:  "",
		},
		{
			name:     "in with multiple values",
			filter:   `data.category in ["A","B"]`,
			startArg: 1,
			wantSQL:  `(data->>'category') IN ($2,$3)`,
			wantArgs: []any{"A", "B"},
		},
		{
			name:     "equals single value",
			filter:   `data.title = "hello"`,
			startArg: 1,
			wantSQL:  `(data->>'title') = $2`,
			wantArgs: []any{"hello"},
		},
		{
			name:     "prefix on date",
			filter:   `data.created_on prefix "2026-04-15"`,
			startArg: 1,
			wantSQL:  `(data->>'created_on') LIKE $2`,
			wantArgs: []any{"2026-04-15%"},
		},
		{
			name:     "two clauses AND'd",
			filter:   `data.category in ["A"], data.title = "x"`,
			startArg: 1,
			wantSQL:  `(data->>'category') IN ($2) AND (data->>'title') = $3`,
			wantArgs: []any{"A", "x"},
		},
		{
			name:       "unknown field rejected",
			filter:     `data.ghost = "x"`,
			startArg:   1,
			wantErrSub: "unknown field",
		},
		{
			name:       "missing data prefix rejected",
			filter:     `category = "A"`,
			startArg:   1,
			wantErrSub: "must start with data.",
		},
		{
			name:       "unsupported operator rejected",
			filter:     `data.category like "A"`,
			startArg:   1,
			wantErrSub: "unsupported filter operator",
		},
		{
			name:       "empty list rejected",
			filter:     `data.category in []`,
			startArg:   1,
			wantErrSub: "empty in()",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pf, err := parseFilterDSL(tc.filter, fields, tc.startArg)
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErrSub, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pf.sql != tc.wantSQL {
				t.Fatalf("sql mismatch:\n  got:  %q\n  want: %q", pf.sql, tc.wantSQL)
			}
			if len(pf.args) != len(tc.wantArgs) {
				t.Fatalf("args len: got %d, want %d (%v vs %v)", len(pf.args), len(tc.wantArgs), pf.args, tc.wantArgs)
			}
			for i := range pf.args {
				if pf.args[i] != tc.wantArgs[i] {
					t.Fatalf("arg[%d]: got %v, want %v", i, pf.args[i], tc.wantArgs[i])
				}
			}
		})
	}
}
