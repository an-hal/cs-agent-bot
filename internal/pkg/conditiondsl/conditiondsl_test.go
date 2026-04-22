package conditiondsl

import (
	"context"
	"testing"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/workday"
)

// testRecord implements Record for testing.
type testRecord struct {
	fields map[string]string
	dates  map[string]time.Time
}

func (r *testRecord) GetField(name string) (string, bool) {
	v, ok := r.fields[name]
	return v, ok
}

func (r *testRecord) GetDateField(name string) (*time.Time, bool) {
	v, ok := r.dates[name]
	if !ok {
		return nil, false
	}
	return &v, true
}

func newEvaluator() *Evaluator {
	return NewEvaluator(workday.NewProvider(""))
}

func TestEvaluate_Empty(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{}

	result, err := e.Evaluate(context.Background(), "", rec)
	if err != nil || !result {
		t.Error("empty condition should return true")
	}

	result, err = e.Evaluate(context.Background(), "-", rec)
	if err != nil || !result {
		t.Error("'-' condition should return true")
	}
}

func TestEvaluate_Equals(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{"stage": "LEAD"}}

	result, err := e.Evaluate(context.Background(), "stage = LEAD", rec)
	if err != nil || !result {
		t.Error("stage = LEAD should match")
	}

	result, err = e.Evaluate(context.Background(), "stage = PROSPECT", rec)
	if err != nil || result {
		t.Error("stage = PROSPECT should not match")
	}
}

func TestEvaluate_EqualsQuoted(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{"payment_status": "Overdue"}}

	result, err := e.Evaluate(context.Background(), "payment_status = 'Overdue'", rec)
	if err != nil || !result {
		t.Error("quoted value should match")
	}
}

func TestEvaluate_NotEquals(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{"stage": "LEAD"}}

	result, err := e.Evaluate(context.Background(), "stage != PROSPECT", rec)
	if err != nil || !result {
		t.Error("stage != PROSPECT should be true when stage=LEAD")
	}
}

func TestEvaluate_GreaterOrEqual(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{"nps_score": "8"}}

	result, err := e.Evaluate(context.Background(), "nps_score >= 8", rec)
	if err != nil || !result {
		t.Error("nps_score >= 8 should match when nps_score=8")
	}

	result, err = e.Evaluate(context.Background(), "nps_score >= 9", rec)
	if err != nil || result {
		t.Error("nps_score >= 9 should not match when nps_score=8")
	}
}

func TestEvaluate_LessOrEqual(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{"days_to_expiry": "5"}}

	result, err := e.Evaluate(context.Background(), "days_to_expiry <= 10", rec)
	if err != nil || !result {
		t.Error("days_to_expiry <= 10 should match when value=5")
	}
}

func TestEvaluate_Between(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{"days_to_expiry": "87"}}

	result, err := e.Evaluate(context.Background(), "days_to_expiry BETWEEN 85 AND 90", rec)
	if err != nil || !result {
		t.Error("87 should be between 85 and 90")
	}

	result, err = e.Evaluate(context.Background(), "days_to_expiry BETWEEN 90 AND 100", rec)
	if err != nil || result {
		t.Error("87 should not be between 90 and 100")
	}
}

func TestEvaluate_In(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{"stage": "LEAD"}}

	result, err := e.Evaluate(context.Background(), "stage IN ('LEAD','PROSPECT')", rec)
	if err != nil || !result {
		t.Error("LEAD should be in the list")
	}

	rec.fields["stage"] = "CLIENT"
	result, err = e.Evaluate(context.Background(), "stage IN ('LEAD','PROSPECT')", rec)
	if err != nil || result {
		t.Error("CLIENT should not be in the list")
	}
}

func TestEvaluate_IsNull(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{}}

	result, err := e.Evaluate(context.Background(), "closing_date IS NULL", rec)
	if err != nil || !result {
		t.Error("missing field should be IS NULL")
	}

	rec.fields["closing_date"] = "2026-01-01"
	result, err = e.Evaluate(context.Background(), "closing_date IS NULL", rec)
	if err != nil || result {
		t.Error("existing field should not be IS NULL")
	}
}

func TestEvaluate_IsNotNull(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{"closing_date": "2026-01-01"}}

	result, err := e.Evaluate(context.Background(), "closing_date IS NOT NULL", rec)
	if err != nil || !result {
		t.Error("existing field should be IS NOT NULL")
	}
}

func TestEvaluate_AndCombinator(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{
		"stage":        "LEAD",
		"bot_active":   "true",
		"days_to_expiry": "50",
	}}

	cond := "stage = LEAD\nAND bot_active = true"
	result, err := e.Evaluate(context.Background(), cond, rec)
	if err != nil || !result {
		t.Error("both conditions should match")
	}

	cond = "stage = LEAD\nAND bot_active = false"
	result, err = e.Evaluate(context.Background(), cond, rec)
	if err != nil || result {
		t.Error("second condition fails, AND should be false")
	}
}

func TestEvaluate_OrCombinator(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{
		"payment_status": "Overdue",
		"stage":          "CLIENT",
	}}

	cond := "payment_status = Overdue\nOR stage = LEAD"
	result, err := e.Evaluate(context.Background(), cond, rec)
	if err != nil || !result {
		t.Error("first OR branch matches, should be true")
	}

	cond = "payment_status = Paid\nOR stage = LEAD"
	result, err = e.Evaluate(context.Background(), cond, rec)
	if err != nil || result {
		t.Error("neither OR branch matches, should be false")
	}
}

func TestEvaluate_FieldNotFound(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{}}

	result, err := e.Evaluate(context.Background(), "nonexistent = foo", rec)
	if err != nil || result {
		t.Error("nonexistent field should not match, no error")
	}
}

func TestEvaluate_MissingFieldIsNull(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{}}

	result, err := e.Evaluate(context.Background(), "missing_field IS NULL", rec)
	if err != nil || !result {
		t.Error("missing field should satisfy IS NULL")
	}
}

func TestEvaluate_EqualsCaseInsensitive(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{"bot_active": "TRUE"}}

	result, err := e.Evaluate(context.Background(), "bot_active = true", rec)
	if err != nil || !result {
		t.Error("= comparison should be case-insensitive")
	}
}

func TestEvaluate_BoolAsString(t *testing.T) {
	e := newEvaluator()
	rec := &testRecord{fields: map[string]string{"onboarding_sent": "false"}}

	result, err := e.Evaluate(context.Background(), "onboarding_sent = FALSE", rec)
	if err != nil || !result {
		t.Error("false = FALSE should match (case-insensitive)")
	}
}
