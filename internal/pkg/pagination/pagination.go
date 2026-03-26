package pagination

import (
	"net/http"
	"strconv"
)

// Params represents pagination parameters using offset/limit convention
type Params struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// Meta represents pagination metadata for API responses
type Meta struct {
	Offset int   `json:"offset"`
	Limit  int   `json:"limit"`
	Total  int64 `json:"total"`
}

const (
	DefaultOffset = 0
	DefaultLimit  = 10
	MaxLimit      = 100
)

// FromRequest extracts pagination parameters from HTTP request query params
// Uses offset/limit instead of page/per_page
func FromRequest(r *http.Request) Params {
	offset := parseIntOr(r.URL.Query().Get("offset"), DefaultOffset)
	limit := parseIntOr(r.URL.Query().Get("limit"), DefaultLimit)

	// Validate boundaries
	if offset < 0 {
		offset = DefaultOffset
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}

	return Params{
		Offset: offset,
		Limit:  limit,
	}
}

// NewMeta creates pagination metadata from params and total count
func NewMeta(params Params, total int64) Meta {
	return Meta{
		Offset: params.Offset,
		Limit:  params.Limit,
		Total:  total,
	}
}

// parseIntOr parses an integer from string or returns default value
func parseIntOr(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}
