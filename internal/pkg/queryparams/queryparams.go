package queryparams

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// HasParam checks if a query parameter exists
func HasParam(r *http.Request, key string) bool {
	return r.URL.Query().Has(key)
}

// GetStringEq gets a string value from key_eq query parameter
func GetStringEq(r *http.Request, key string) string {
	return r.URL.Query().Get(key + "_eq")
}

// GetUUIDEq gets and validates a UUID from key_eq query parameter
// Returns error if parameter is missing or invalid UUID format
func GetUUIDEq(r *http.Request, key string) (uuid.UUID, error) {
	val := GetStringEq(r, key)
	if val == "" {
		return uuid.Nil, fmt.Errorf("%s_eq parameter is required", key)
	}

	parsed, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s_eq must be a valid UUID", key)
	}

	return parsed, nil
}

// GetIntEq gets an integer value from key_eq query parameter
func GetIntEq(r *http.Request, key string) (int, bool) {
	val := r.URL.Query().Get(key + "_eq")
	if val == "" {
		return 0, false
	}

	var result int
	_, err := fmt.Sscanf(val, "%d", &result)
	if err != nil {
		return 0, false
	}

	return result, true
}

// GetFloatEq gets a float64 value from key_eq query parameter
func GetFloatEq(r *http.Request, key string) (float64, bool) {
	val := r.URL.Query().Get(key + "_eq")
	if val == "" {
		return 0, false
	}

	var result float64
	_, err := fmt.Sscanf(val, "%f", &result)
	if err != nil {
		return 0, false
	}

	return result, true
}

// RequireExactlyOne validates that exactly one of the provided params is true
// Returns error if zero or more than one parameter is provided
// Example: RequireExactlyOne(map[string]bool{"id_eq": true, "external_id_eq": false})
func RequireExactlyOne(params map[string]bool) error {
	count := 0
	var provided []string

	for key, hasValue := range params {
		if hasValue {
			count++
			provided = append(provided, key)
		}
	}

	if count == 0 {
		keys := make([]string, 0, len(params))
		for key := range params {
			keys = append(keys, key)
		}
		return fmt.Errorf("exactly one of %v must be provided", keys)
	}

	if count > 1 {
		return fmt.Errorf("only one identifier allowed, got: %v", provided)
	}

	return nil
}

// GetStringLike gets a string value from key_like query parameter for ILIKE search.
func GetStringLike(r *http.Request, key string) string {
	return r.URL.Query().Get(key + "_like")
}

// GetString gets a raw string value from the query parameter (no suffix).
func GetString(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

// RequireAtLeastOne validates that at least one of the provided params is true
// Returns error if none are provided
func RequireAtLeastOne(params map[string]bool) error {
	for _, hasValue := range params {
		if hasValue {
			return nil
		}
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	return fmt.Errorf("at least one of %v must be provided", keys)
}
