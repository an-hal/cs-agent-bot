package repository

import (
	"strings"
	"testing"
)

func TestBuildConditionClause_RejectsBadField(t *testing.T) {
	bads := []string{
		"id; DROP TABLE clients",
		"custom_fields.foo'; --",
		"",
		"name space",
		"--",
	}
	for _, f := range bads {
		t.Run(f, func(t *testing.T) {
			_, err := buildConditionClause(QueryCondition{Field: f, Op: "=", Value: "x"})
			if err == nil {
				t.Fatalf("expected rejection for %q", f)
			}
		})
	}
}

func TestBuildConditionClause_RejectsBadOp(t *testing.T) {
	_, err := buildConditionClause(QueryCondition{Field: "stage", Op: "DROP", Value: "x"})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected op rejection, got %v", err)
	}
}

func TestBuildConditionClause_AllowsCoreField(t *testing.T) {
	_, err := buildConditionClause(QueryCondition{Field: "stage", Op: "=", Value: "CLIENT"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestBuildConditionClause_AllowsJSONPath(t *testing.T) {
	_, err := buildConditionClause(QueryCondition{Field: "custom_fields.nps_score", Op: ">=", Value: float64(8)})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestBuildConditionClause_BetweenRequiresArray(t *testing.T) {
	_, err := buildConditionClause(QueryCondition{Field: "days_to_expiry", Op: "between", Value: 5})
	if err == nil {
		t.Fatalf("expected error")
	}
	_, err = buildConditionClause(QueryCondition{Field: "days_to_expiry", Op: "between", Value: []any{0, 30}})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestBuildConditionClause_InRequiresArray(t *testing.T) {
	_, err := buildConditionClause(QueryCondition{Field: "stage", Op: "in", Value: "CLIENT"})
	if err == nil {
		t.Fatalf("expected error")
	}
	_, err = buildConditionClause(QueryCondition{Field: "stage", Op: "in", Value: []any{"CLIENT", "PROSPECT"}})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestIsSafeFieldName(t *testing.T) {
	good := []string{"stage", "company_name", "custom_fields.nps_score", "a.b.c"}
	bad := []string{"", "name space", "drop;", "x'y", "--"}
	for _, s := range good {
		if !isSafeFieldName(s) {
			t.Fatalf("expected %q safe", s)
		}
	}
	for _, s := range bad {
		if isSafeFieldName(s) {
			t.Fatalf("expected %q unsafe", s)
		}
	}
}
