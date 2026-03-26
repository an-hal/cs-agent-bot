package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/go-playground/validator/v10"
)

// Validator wraps go-playground/validator with custom configuration
type Validator struct {
	validate *validator.Validate
}

// New creates a new Validator instance with custom configuration
func New() *Validator {
	v := validator.New()

	// Use JSON tag names for field names in error messages
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return fld.Name
		}
		if name == "" {
			return fld.Name
		}
		return name
	})

	return &Validator{validate: v}
}

// Validate validates a struct and returns *apperror.AppError if validation fails
func (v *Validator) Validate(s interface{}) error {
	err := v.validate.Struct(s)
	if err == nil {
		return nil
	}

	validationErrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return apperror.ValidationError(err.Error())
	}

	var fields []apperror.FieldError
	for _, e := range validationErrs {
		fields = append(fields, apperror.FieldError{
			Field:   e.Field(),
			Message: formatErrorMessage(e),
		})
	}

	return apperror.ValidationErrorWithFields("Validation failed", fields)
}

// formatErrorMessage creates human-readable error messages
func formatErrorMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("Must be at least %s characters", e.Param())
		}
		return fmt.Sprintf("Must be at least %s", e.Param())
	case "max":
		if e.Kind() == reflect.String {
			return fmt.Sprintf("Must be at most %s characters", e.Param())
		}
		return fmt.Sprintf("Must be at most %s", e.Param())
	case "len":
		return fmt.Sprintf("Must be exactly %s characters", e.Param())
	case "gte":
		return fmt.Sprintf("Must be greater than or equal to %s", e.Param())
	case "lte":
		return fmt.Sprintf("Must be less than or equal to %s", e.Param())
	case "gt":
		return fmt.Sprintf("Must be greater than %s", e.Param())
	case "lt":
		return fmt.Sprintf("Must be less than %s", e.Param())
	case "oneof":
		return fmt.Sprintf("Must be one of: %s", e.Param())
	case "uuid":
		return "Must be a valid UUID"
	case "url":
		return "Must be a valid URL"
	case "alphanum":
		return "Must contain only alphanumeric characters"
	case "alpha":
		return "Must contain only alphabetic characters"
	case "numeric":
		return "Must be a numeric value"
	case "e164":
		return "Must be a valid E.164 phone number"
	default:
		return fmt.Sprintf("Failed validation: %s", e.Tag())
	}
}

// RegisterValidation registers a custom validation function
func (v *Validator) RegisterValidation(tag string, fn validator.Func) error {
	return v.validate.RegisterValidation(tag, fn)
}
