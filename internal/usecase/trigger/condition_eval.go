package trigger

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ConditionNode represents a single node in the condition expression tree.
// Supported forms:
//
//	{"and": [...]}                                    — all children must match
//	{"or": [...]}                                     — at least one child must match
//	{"not": {...}}                                    — child must NOT match
//	{"field": "name", "op": "lt", "value": 40}        — compare a client field
//	{"flag": "name", "op": "not_set"}                 — check a flag value
type ConditionNode struct {
	// Logical operators
	And []json.RawMessage `json:"and,omitempty"`
	Or  []json.RawMessage `json:"or,omitempty"`
	Not json.RawMessage   `json:"not,omitempty"`

	// Field comparison
	Field string      `json:"field,omitempty"`
	Flag  string      `json:"flag,omitempty"`
	Op    string      `json:"op,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

// EvaluateCondition evaluates a JSON condition expression against a ClientContext.
func EvaluateCondition(conditionJSON json.RawMessage, ctx *ClientContext) (bool, error) {
	var node ConditionNode
	if err := json.Unmarshal(conditionJSON, &node); err != nil {
		return false, fmt.Errorf("parse condition: %w", err)
	}
	return evalNode(&node, ctx)
}

func evalNode(node *ConditionNode, ctx *ClientContext) (bool, error) {
	// Logical AND
	if len(node.And) > 0 {
		for _, child := range node.And {
			result, err := EvaluateCondition(child, ctx)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	}

	// Logical OR
	if len(node.Or) > 0 {
		for _, child := range node.Or {
			result, err := EvaluateCondition(child, ctx)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	}

	// Logical NOT
	if node.Not != nil {
		result, err := EvaluateCondition(node.Not, ctx)
		if err != nil {
			return false, err
		}
		return !result, nil
	}

	// Flag check
	if node.Flag != "" {
		return evalFlagCondition(node, ctx)
	}

	// Field comparison
	if node.Field != "" {
		return evalFieldCondition(node, ctx)
	}

	return false, fmt.Errorf("invalid condition node: no field, flag, or logical operator")
}

func evalFlagCondition(node *ConditionNode, ctx *ClientContext) (bool, error) {
	flagVal, found := ctx.GetFlag(node.Flag)
	if !found {
		return false, fmt.Errorf("unknown flag: %s", node.Flag)
	}

	switch node.Op {
	case "set":
		return flagVal, nil
	case "not_set":
		return !flagVal, nil
	default:
		return false, fmt.Errorf("unsupported flag operator: %s", node.Op)
	}
}

func evalFieldCondition(node *ConditionNode, ctx *ClientContext) (bool, error) {
	fieldVal, found := ctx.GetField(node.Field)
	if !found {
		return false, fmt.Errorf("unknown field: %s", node.Field)
	}

	switch node.Op {
	case "eq":
		return compareEq(fieldVal, node.Value), nil
	case "neq":
		return !compareEq(fieldVal, node.Value), nil
	case "lt":
		return compareNumeric(fieldVal, node.Value, func(a, b float64) bool { return a < b })
	case "lte":
		return compareNumeric(fieldVal, node.Value, func(a, b float64) bool { return a <= b })
	case "gt":
		return compareNumeric(fieldVal, node.Value, func(a, b float64) bool { return a > b })
	case "gte":
		return compareNumeric(fieldVal, node.Value, func(a, b float64) bool { return a >= b })
	case "in":
		return compareIn(fieldVal, node.Value), nil
	case "not_in":
		return !compareIn(fieldVal, node.Value), nil
	case "is_empty":
		return isEmpty(fieldVal), nil
	case "not_empty":
		return !isEmpty(fieldVal), nil
	case "is_true":
		return toBool(fieldVal), nil
	case "is_false":
		return !toBool(fieldVal), nil
	default:
		return false, fmt.Errorf("unsupported field operator: %s", node.Op)
	}
}

func compareEq(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func compareNumeric(a, b interface{}, cmp func(float64, float64) bool) (bool, error) {
	aNum, err := toFloat64(a)
	if err != nil {
		return false, fmt.Errorf("left operand: %w", err)
	}
	bNum, err := toFloat64(b)
	if err != nil {
		return false, fmt.Errorf("right operand: %w", err)
	}
	return cmp(aNum, bNum), nil
}

func compareIn(fieldVal, listVal interface{}) bool {
	list, ok := listVal.([]interface{})
	if !ok {
		return false
	}
	fieldStr := fmt.Sprintf("%v", fieldVal)
	for _, item := range list {
		if fmt.Sprintf("%v", item) == fieldStr {
			return true
		}
	}
	return false
}

func isEmpty(val interface{}) bool {
	if val == nil {
		return true
	}
	s, ok := val.(string)
	if ok {
		return strings.TrimSpace(s) == ""
	}
	b, ok := val.(bool)
	if ok {
		return !b
	}
	return false
}

func toBool(val interface{}) bool {
	switch v := val.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case float64:
		return v != 0
	case string:
		return v != "" && v != "false" && v != "0"
	default:
		return val != nil
	}
}

func toFloat64(val interface{}) (float64, error) {
	switch v := val.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", val)
	}
}
