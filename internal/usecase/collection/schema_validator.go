package collection

import (
	"context"
	"fmt"
	"math"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// isoDateRe matches YYYY-MM-DD; datetime is handled separately via time.Parse.
var isoDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// ValidateRecordData checks `data` against the collection's field schema.
// Returns (per-field errors, overall error). `fieldErrs` has one entry per
// offending key; the overall error summarizes so API responses stay consistent.
//
// If strict is true, keys in data that are not declared in fields are rejected.
func ValidateRecordData(
	ctx context.Context,
	fields []entity.CollectionField,
	data map[string]any,
	strict bool,
	linkClientCheck func(ctx context.Context, id string) (bool, error),
) (map[string]string, error) {
	fieldErrs := map[string]string{}
	byKey := make(map[string]entity.CollectionField, len(fields))
	for _, f := range fields {
		byKey[f.Key] = f
	}

	if strict {
		for k := range data {
			if _, ok := byKey[k]; !ok {
				fieldErrs[k] = "unknown field"
			}
		}
	}

	for _, f := range fields {
		v, present := data[f.Key]
		if !present || v == nil {
			if f.Required {
				fieldErrs[f.Key] = "required"
			}
			continue
		}
		if msg := validateOne(ctx, f, v, linkClientCheck); msg != "" {
			fieldErrs[f.Key] = msg
		}
	}

	if len(fieldErrs) > 0 {
		return fieldErrs, fmt.Errorf("validation failed for %d field(s)", len(fieldErrs))
	}
	return nil, nil
}

// validateOne returns an error message if the value is invalid, or "" if OK.
//
//nolint:gocyclo // one switch per field type is the clearest shape here
func validateOne(
	ctx context.Context,
	f entity.CollectionField,
	v any,
	linkClientCheck func(ctx context.Context, id string) (bool, error),
) string {
	switch f.Type {
	case entity.ColFieldText, entity.ColFieldTextarea, entity.ColFieldFile:
		s, ok := v.(string)
		if !ok {
			return "must be string"
		}
		if ml, ok := asInt(f.Options["maxLength"]); ok && ml > 0 && len(s) > ml {
			return fmt.Sprintf("exceeds maxLength %d", ml)
		}
		return ""

	case entity.ColFieldNumber:
		n, ok := asFloat(v)
		if !ok {
			return "must be number"
		}
		if mi, ok := asFloat(f.Options["min"]); ok && n < mi {
			return fmt.Sprintf("below min %v", mi)
		}
		if ma, ok := asFloat(f.Options["max"]); ok && n > ma {
			return fmt.Sprintf("above max %v", ma)
		}
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return "not a finite number"
		}
		return ""

	case entity.ColFieldBoolean:
		if _, ok := v.(bool); !ok {
			return "must be boolean"
		}
		return ""

	case entity.ColFieldDate:
		s, ok := v.(string)
		if !ok {
			return "must be ISO date string"
		}
		if !isoDateRe.MatchString(s) {
			return "must match YYYY-MM-DD"
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return "invalid date"
		}
		return ""

	case entity.ColFieldDateTime:
		s, ok := v.(string)
		if !ok {
			return "must be ISO datetime string"
		}
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			return "must be RFC3339 datetime"
		}
		return ""

	case entity.ColFieldEnum:
		s, ok := v.(string)
		if !ok {
			return "must be string"
		}
		return checkChoice(f.Options, s)

	case entity.ColFieldMultiEnum:
		arr, ok := v.([]any)
		if !ok {
			return "must be array"
		}
		for _, it := range arr {
			s, ok := it.(string)
			if !ok {
				return "items must be strings"
			}
			if msg := checkChoice(f.Options, s); msg != "" {
				return msg
			}
		}
		return ""

	case entity.ColFieldURL:
		s, ok := v.(string)
		if !ok {
			return "must be string"
		}
		u, err := url.Parse(s)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "invalid URL"
		}
		return ""

	case entity.ColFieldEmail:
		s, ok := v.(string)
		if !ok {
			return "must be string"
		}
		if _, err := mail.ParseAddress(s); err != nil {
			return "invalid email"
		}
		return ""

	case entity.ColFieldLinkClient:
		s, ok := v.(string)
		if !ok {
			return "must be string (client id)"
		}
		if linkClientCheck == nil {
			return ""
		}
		ok2, err := linkClientCheck(ctx, s)
		if err != nil {
			return "link_client lookup failed"
		}
		if !ok2 {
			return "client not found in workspace"
		}
		return ""
	}
	return "unsupported type"
}

// checkChoice verifies s is one of options.choices (if provided).
func checkChoice(options map[string]any, s string) string {
	raw, ok := options["choices"]
	if !ok {
		return ""
	}
	arr, ok := raw.([]any)
	if !ok {
		return ""
	}
	for _, c := range arr {
		if cs, ok := c.(string); ok && cs == s {
			return ""
		}
	}
	return "not in allowed choices"
}

// asFloat accepts float64, json.Number, or a numeric string.
func asFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// asInt accepts float64 or int-like values (JSON decodes numbers as float64).
func asInt(v any) (int, bool) {
	if f, ok := asFloat(v); ok {
		return int(f), true
	}
	return 0, false
}
