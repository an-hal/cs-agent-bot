package master_data

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

// ValidateCustomFields checks each provided custom field against its definition.
// When `enforceRequired` is true (e.g. on Create), missing required fields
// produce a validation error.
func ValidateCustomFields(defs []entity.CustomFieldDefinition, fields map[string]any, enforceRequired bool) error {
	defsByKey := map[string]*entity.CustomFieldDefinition{}
	for i := range defs {
		defsByKey[defs[i].FieldKey] = &defs[i]
	}

	if enforceRequired {
		for i := range defs {
			d := &defs[i]
			if !d.IsRequired {
				continue
			}
			if _, ok := fields[d.FieldKey]; !ok {
				return apperror.ValidationError(fmt.Sprintf("custom field %q is required", d.FieldKey))
			}
		}
	}

	for key, val := range fields {
		def, ok := defsByKey[key]
		if !ok {
			// Unknown keys are tolerated — workflow nodes may write fields not in defs.
			continue
		}
		if err := validateOne(def, val); err != nil {
			return err
		}
	}
	return nil
}

func validateOne(def *entity.CustomFieldDefinition, val any) error {
	if val == nil {
		if def.IsRequired {
			return apperror.ValidationError(fmt.Sprintf("custom field %q is required", def.FieldKey))
		}
		return nil
	}
	switch def.FieldType {
	case entity.FieldTypeText:
		s, ok := val.(string)
		if !ok {
			return mismatch(def, "text")
		}
		if def.RegexPattern != "" {
			re, err := regexp.Compile(def.RegexPattern)
			if err == nil && !re.MatchString(s) {
				return apperror.ValidationError(fmt.Sprintf("custom field %q does not match pattern", def.FieldKey))
			}
		}
	case entity.FieldTypeNumber:
		f, ok := toFloat(val)
		if !ok {
			return mismatch(def, "number")
		}
		if def.MinValue != nil && f < *def.MinValue {
			return apperror.ValidationError(fmt.Sprintf("custom field %q below min", def.FieldKey))
		}
		if def.MaxValue != nil && f > *def.MaxValue {
			return apperror.ValidationError(fmt.Sprintf("custom field %q above max", def.FieldKey))
		}
	case entity.FieldTypeBoolean:
		if _, ok := val.(bool); !ok {
			return mismatch(def, "boolean")
		}
	case entity.FieldTypeDate:
		s, ok := val.(string)
		if !ok {
			return mismatch(def, "date")
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			if _, err2 := time.Parse(time.RFC3339, s); err2 != nil {
				return apperror.ValidationError(fmt.Sprintf("custom field %q is not a valid date", def.FieldKey))
			}
		}
	case entity.FieldTypeSelect:
		s, ok := val.(string)
		if !ok {
			return mismatch(def, "select")
		}
		opts := def.SelectOptions()
		found := false
		for _, opt := range opts {
			if opt == s {
				found = true
				break
			}
		}
		if !found {
			return apperror.ValidationError(fmt.Sprintf("custom field %q value %q not in options", def.FieldKey, s))
		}
	case entity.FieldTypeURL:
		s, ok := val.(string)
		if !ok {
			return mismatch(def, "url")
		}
		if _, err := url.ParseRequestURI(s); err != nil {
			return apperror.ValidationError(fmt.Sprintf("custom field %q is not a valid URL", def.FieldKey))
		}
	case entity.FieldTypeEmail:
		s, ok := val.(string)
		if !ok {
			return mismatch(def, "email")
		}
		if _, err := mail.ParseAddress(s); err != nil {
			return apperror.ValidationError(fmt.Sprintf("custom field %q is not a valid email", def.FieldKey))
		}
	}
	return nil
}

func mismatch(def *entity.CustomFieldDefinition, want string) error {
	return apperror.ValidationError(fmt.Sprintf("custom field %q expected %s", def.FieldKey, want))
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}
