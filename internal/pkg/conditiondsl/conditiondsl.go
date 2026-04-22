// Package conditiondsl evaluates rule condition strings against a record.
//
// Supported syntax:
//
//	field = value          — equality
//	field != value         — inequality
//	field >= value         — greater than or equal
//	field <= value         — less than or equal
//	field BETWEEN low AND high
//	field IN ('a','b','c')
//	field IS NULL / IS NOT NULL
//	AND / OR combinators (newline-separated)
//	isWorkingDay(TODAY()) = TRUE
//	workingDaysSince(field) >= N
package conditiondsl

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/workday"
)

// Record is the interface satisfied by master_data rows. The condition DSL
// reads field values through this interface so it stays decoupled from the
// concrete entity.
type Record interface {
	// GetField returns the value of a core or custom field.
	// Returns ("", false) when the field does not exist.
	GetField(name string) (string, bool)
	// GetDateField returns a date-typed field value.
	// Returns (nil, false) when the field does not exist or is not a date.
	GetDateField(name string) (*time.Time, bool)
}

// Evaluator holds dependencies for condition evaluation.
type Evaluator struct {
	Workday *workday.Provider
}

// NewEvaluator creates an evaluator with the given workday provider.
func NewEvaluator(wp *workday.Provider) *Evaluator {
	return &Evaluator{Workday: wp}
}

// Evaluate evaluates a condition string against a record.
// Empty string or "-" always returns true.
func (e *Evaluator) Evaluate(ctx context.Context, condition string, rec Record) (bool, error) {
	condition = strings.TrimSpace(condition)
	if condition == "" || condition == "-" {
		return true, nil
	}

	// Check for OR combinators first (any must be true).
	if strings.Contains(condition, "\nOR ") {
		return e.evalOr(ctx, condition, rec)
	}

	// Check for AND combinators (all must be true).
	if strings.Contains(condition, "\nAND ") {
		return e.evalAnd(ctx, condition, rec)
	}

	return e.evalSingle(ctx, strings.TrimSpace(condition), rec)
}

func (e *Evaluator) evalOr(ctx context.Context, cond string, rec Record) (bool, error) {
	parts := strings.Split(cond, "\nOR ")
	for _, p := range parts {
		result, err := e.Evaluate(ctx, strings.TrimSpace(p), rec)
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}
	}
	return false, nil
}

func (e *Evaluator) evalAnd(ctx context.Context, cond string, rec Record) (bool, error) {
	parts := strings.Split(cond, "\nAND ")
	for _, p := range parts {
		result, err := e.Evaluate(ctx, strings.TrimSpace(p), rec)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}
	return true, nil
}

func (e *Evaluator) evalSingle(ctx context.Context, expr string, rec Record) (bool, error) {
	// Special function: isWorkingDay(TODAY())
	if strings.Contains(expr, "isWorkingDay") {
		return e.evalIsWorkingDay(ctx, expr)
	}

	// Special function: workingDaysSince(field)
	if strings.Contains(expr, "workingDaysSince") {
		return e.evalWorkingDaysSince(ctx, expr, rec)
	}

	// IS NULL / IS NOT NULL
	if strings.HasSuffix(expr, "IS NULL") {
		field := strings.TrimSpace(strings.TrimSuffix(expr, "IS NULL"))
		val, found := rec.GetField(field)
		return !found || val == "", nil
	}
	if strings.HasSuffix(expr, "IS NOT NULL") {
		field := strings.TrimSpace(strings.TrimSuffix(expr, "IS NOT NULL"))
		val, found := rec.GetField(field)
		return found && val != "", nil
	}

	// BETWEEN
	if strings.Contains(expr, " BETWEEN ") {
		return e.evalBetween(expr, rec)
	}

	// IN
	if strings.Contains(expr, " IN ") {
		return e.evalIn(expr, rec)
	}

	// Standard comparisons: =, !=, >=, <=
	for _, op := range []string{"!=", ">=", "<=", "="} {
		idx := strings.Index(expr, " "+op+" ")
		if idx == -1 {
			continue
		}
		field := strings.TrimSpace(expr[:idx])
		value := strings.TrimSpace(expr[idx+len(op)+2:])
		value = unquote(value)

		fieldVal, found := rec.GetField(field)
		if !found {
			return false, nil
		}

		return compareOp(fieldVal, op, value)
	}

	return false, fmt.Errorf("conditiondsl: unparseable expression: %q", expr)
}

func (e *Evaluator) evalIsWorkingDay(ctx context.Context, expr string) (bool, error) {
	result := e.Workday.IsWorkingDay(ctx, time.Now())

	// Check if expression includes = TRUE/FALSE
	if strings.Contains(expr, "=") {
		parts := strings.SplitN(expr, "=", 2)
		expected := strings.TrimSpace(unquote(parts[1]))
		return fmt.Sprintf("%v", result) == expected, nil
	}
	return result, nil
}

func (e *Evaluator) evalWorkingDaysSince(ctx context.Context, expr string, rec Record) (bool, error) {
	// Extract field name from workingDaysSince(field_name)
	open := strings.Index(expr, "(")
	close := strings.Index(expr, ")")
	if open == -1 || close == -1 {
		return false, fmt.Errorf("conditiondsl: malformed workingDaysSince: %q", expr)
	}
	fieldName := strings.TrimSpace(expr[open+1 : close])

	dateVal, found := rec.GetDateField(fieldName)
	if !found || dateVal == nil {
		return false, nil
	}

	wd := e.Workday.WorkingDaysSince(ctx, *dateVal, time.Now())

	// Parse the comparison after the closing paren
	remainder := strings.TrimSpace(expr[close+1:])
	if remainder == "" {
		return wd > 0, nil
	}

	// e.g. ">= 3"
	for _, op := range []string{"!=", ">=", "<=", "=", ">", "<"} {
		if strings.HasPrefix(remainder, op) {
			thresholdStr := strings.TrimSpace(remainder[len(op):])
			threshold, err := strconv.ParseFloat(thresholdStr, 64)
			if err != nil {
				return false, fmt.Errorf("conditiondsl: workingDaysSince threshold: %w", err)
			}
			return compareNumeric(float64(wd), op, threshold), nil
		}
	}

	return false, fmt.Errorf("conditiondsl: unparseable workingDaysSince remainder: %q", remainder)
}

func (e *Evaluator) evalBetween(expr string, rec Record) (bool, error) {
	// "field BETWEEN low AND high"
	parts := strings.SplitN(expr, " BETWEEN ", 2)
	if len(parts) != 2 {
		return false, fmt.Errorf("conditiondsl: malformed BETWEEN: %q", expr)
	}
	field := strings.TrimSpace(parts[0])
	rangePart := strings.TrimSpace(parts[1])

	andParts := strings.SplitN(rangePart, " AND ", 2)
	if len(andParts) != 2 {
		return false, fmt.Errorf("conditiondsl: malformed BETWEEN range: %q", rangePart)
	}
	lowStr := strings.TrimSpace(unquote(andParts[0]))
	highStr := strings.TrimSpace(unquote(andParts[1]))

	fieldVal, found := rec.GetField(field)
	if !found {
		return false, nil
	}

	fieldNum, fieldIsNum := toFloat(fieldVal)
	lowNum, lowIsNum := toFloat(lowStr)
	highNum, highIsNum := toFloat(highStr)

	if fieldIsNum && lowIsNum && highIsNum {
		return fieldNum >= lowNum && fieldNum <= highNum, nil
	}

	// String comparison fallback
	return fieldVal >= lowStr && fieldVal <= highStr, nil
}

func (e *Evaluator) evalIn(expr string, rec Record) (bool, error) {
	// "field IN ('a','b','c')"
	parts := strings.SplitN(expr, " IN ", 2)
	if len(parts) != 2 {
		return false, fmt.Errorf("conditiondsl: malformed IN: %q", expr)
	}
	field := strings.TrimSpace(parts[0])
	listStr := strings.TrimSpace(parts[1])

	if listStr[0] != '(' || listStr[len(listStr)-1] != ')' {
		return false, fmt.Errorf("conditiondsl: malformed IN list: %q", listStr)
	}
	listStr = listStr[1 : len(listStr)-1]

	fieldVal, found := rec.GetField(field)
	if !found {
		return false, nil
	}

	for _, item := range strings.Split(listStr, ",") {
		if strings.TrimSpace(unquote(item)) == fieldVal {
			return true, nil
		}
	}
	return false, nil
}

// compareOp compares fieldVal against value using the given operator.
func compareOp(fieldVal, op, value string) (bool, error) {
	fieldNum, fieldIsNum := toFloat(fieldVal)
	valNum, valIsNum := toFloat(value)

	switch op {
	case "=":
		if fieldIsNum && valIsNum {
			return fieldNum == valNum, nil
		}
		return strings.EqualFold(fieldVal, value), nil
	case "!=":
		if fieldIsNum && valIsNum {
			return fieldNum != valNum, nil
		}
		return !strings.EqualFold(fieldVal, value), nil
	case ">=":
		if fieldIsNum && valIsNum {
			return fieldNum >= valNum, nil
		}
		return fieldVal >= value, nil
	case "<=":
		if fieldIsNum && valIsNum {
			return fieldNum <= valNum, nil
		}
		return fieldVal <= value, nil
	default:
		return false, fmt.Errorf("conditiondsl: unsupported operator: %s", op)
	}
}

func compareNumeric(fieldVal float64, op string, threshold float64) bool {
	switch op {
	case ">=":
		return fieldVal >= threshold
	case "<=":
		return fieldVal <= threshold
	case ">":
		return fieldVal > threshold
	case "<":
		return fieldVal < threshold
	case "=":
		return fieldVal == threshold
	case "!=":
		return fieldVal != threshold
	default:
		return false
	}
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func toFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}
