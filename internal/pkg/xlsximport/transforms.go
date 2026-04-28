// Phase B: type-driven cell transforms.
//
// Each Transform* function takes a raw cell string and returns a coerced typed
// value or a *TransformError that the parser converts into a CellError.
// Empty input is treated as "no value" — Transform* returns the zero value with
// nil error so callers can distinguish "missing" from "invalid".
//
// Locale notes (Indonesian xlsx exports are common):
//   - Numbers may use ',' as decimal separator and '.' as thousands separator.
//   - Currency strings often have "Rp" or "IDR" prefix and trailing decimals.
//   - Booleans appear as Yes/No, Y/N, 1/0, true/false, iya/tidak, aktif/nonaktif.
//   - Dates appear as YYYY-MM-DD, DD/MM/YYYY, MM/DD/YYYY, Excel serial, or RFC3339.
//   - Phone numbers may render as "6.288E+12" (Excel scientific notation) or
//     contain spaces/dashes/+ prefix.

package xlsximport

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/xuri/excelize/v2"
)

// TransformError is what each Transform* returns on bad input. The parser
// surfaces it as a CellError tagged with row + column + target_key.
type TransformError struct {
	Code    string
	Message string
}

func (e *TransformError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func newErr(code, msg string) *TransformError { return &TransformError{Code: code, Message: msg} }

// TransformText trims whitespace and optionally enforces a max length. Empty
// input returns ("", nil). When max > 0 and the trimmed value exceeds it, an
// error is returned (no silent truncation — the user should fix this in the
// wizard).
func TransformText(raw string, max int) (string, *TransformError) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if max > 0 && len(s) > max {
		return s, newErr("too_long", fmt.Sprintf("value exceeds %d characters (got %d)", max, len(s)))
	}
	return s, nil
}

// TransformNumber parses an int/float, accepting both '.' and ',' as decimal
// separators and stripping thousands separators (the OTHER one). "12.500,75"
// or "12,500.75" both parse to 12500.75. Returns 0 for empty input.
func TransformNumber(raw string) (float64, *TransformError) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, nil
	}
	// Strip currency-ish prefixes/suffixes and spaces.
	s = stripUnits(s)
	if s == "" {
		return 0, newErr("invalid_number", "empty after stripping units")
	}

	hasDot := strings.Contains(s, ".")
	hasComma := strings.Contains(s, ",")
	switch {
	case hasDot && hasComma:
		// The rightmost separator is the decimal mark; the other is thousands.
		if strings.LastIndex(s, ",") > strings.LastIndex(s, ".") {
			s = strings.ReplaceAll(s, ".", "")
			s = strings.Replace(s, ",", ".", 1)
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	case hasComma:
		// Single comma. If followed by exactly 3 digits and no other comma,
		// it's likely a thousands separator (US style: "12,500"). Otherwise
		// treat as decimal (locale-id style: "12,5").
		if commaCount := strings.Count(s, ","); commaCount == 1 {
			parts := strings.Split(s, ",")
			if len(parts[1]) == 3 && allDigits(parts[1]) {
				s = strings.ReplaceAll(s, ",", "")
			} else {
				s = strings.Replace(s, ",", ".", 1)
			}
		} else {
			// Multiple commas → all are thousands separators.
			s = strings.ReplaceAll(s, ",", "")
		}
	case hasDot && strings.Count(s, ".") > 1:
		// "12.500.000" — multiple dots, no comma → Indonesian thousands separator.
		s = strings.ReplaceAll(s, ".", "")
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, newErr("invalid_number", "expected number, got "+strconv.Quote(raw))
	}
	return f, nil
}

// TransformInt parses an integer the same way TransformNumber does, but
// rejects fractional values.
func TransformInt(raw string) (int, *TransformError) {
	f, err := TransformNumber(raw)
	if err != nil {
		return 0, err
	}
	if f != float64(int(f)) {
		return 0, newErr("invalid_integer", "expected integer, got "+strconv.Quote(raw))
	}
	return int(f), nil
}

// TransformCurrency strips currency symbols ("Rp", "$", "USD", "IDR") and
// parses to a float using the same logic as TransformNumber. Many xlsx exports
// store money as text like "Rp 12.000.000".
func TransformCurrency(raw string) (float64, *TransformError) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, nil
	}
	for _, sym := range []string{"Rp.", "Rp", "IDR", "USD", "SGD", "$", "€", "£"} {
		s = strings.TrimSpace(strings.ReplaceAll(s, sym, ""))
	}
	if s == "" {
		return 0, newErr("invalid_currency", "empty after stripping symbol")
	}
	return TransformNumber(s)
}

// dateLayouts is the ordered list TransformDate tries; first-match wins. Order
// matters: more specific layouts (with time) first, then date-only.
var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:4:5",
	"2006-01-02",
	"02/01/2006", // DD/MM/YYYY (locale-id default)
	"01/02/2006", // MM/DD/YYYY
	"2/1/2006",
	"1/2/2006",
	"02-01-2006",
	"01-02-2006",
	"2006/01/02",
}

// TransformDate parses a wide range of date formats. Empty input → zero time
// with nil error.
func TransformDate(raw string) (time.Time, *TransformError) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return time.Time{}, nil
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	// Excel serial number (post-1900 epoch starts at 25569).
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 25569 {
		if t, err := excelize.ExcelDateToTime(f, false); err == nil {
			return t, nil
		}
	}
	return time.Time{}, newErr("invalid_date", "unrecognized date format: "+strconv.Quote(raw))
}

// TransformDatePtr is TransformDate but returns a *time.Time (nil for empty).
func TransformDatePtr(raw string) (*time.Time, *TransformError) {
	t, err := TransformDate(raw)
	if err != nil || t.IsZero() {
		return nil, err
	}
	return &t, nil
}

// boolTrue / boolFalse cover English + Indonesian + Y/N + 1/0 variants.
var (
	boolTrue  = map[string]bool{"yes": true, "y": true, "true": true, "t": true, "1": true, "iya": true, "ya": true, "aktif": true, "active": true, "on": true}
	boolFalse = map[string]bool{"no": true, "n": true, "false": true, "f": true, "0": true, "tidak": true, "nonaktif": true, "inactive": true, "off": true}
)

// TransformBool accepts a wide variety of truthy/falsy strings. Empty input
// returns (def, nil) so callers can express "default when blank".
func TransformBool(raw string, def bool) (bool, *TransformError) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return def, nil
	}
	if boolTrue[s] {
		return true, nil
	}
	if boolFalse[s] {
		return false, nil
	}
	return def, newErr("invalid_boolean", "expected yes/no, got "+strconv.Quote(raw))
}

// TransformEnum case-insensitively matches the input against the allowed
// options and returns the canonical option string. Returns "" for empty input.
func TransformEnum(raw string, options []string) (string, *TransformError) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if len(options) == 0 {
		return s, nil
	}
	low := strings.ToLower(s)
	for _, o := range options {
		if strings.EqualFold(o, s) || strings.ToLower(o) == low {
			return o, nil
		}
	}
	return s, newErr("invalid_enum", fmt.Sprintf("value %q not in [%s]", raw, strings.Join(options, ", ")))
}

// TransformPhone normalizes WhatsApp / phone numbers. Strips non-digits,
// expands Excel scientific notation ("6.28898E+12" → "628898000000"), and
// returns digits-only. Empty input → ("", nil). Returns an error if the
// resulting digit string is shorter than 8 (likely junk).
func TransformPhone(raw string) (string, *TransformError) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	// Excel scientific notation? "6.28898E+12" → 6288980000000
	if strings.ContainsAny(s, "Ee") {
		if f, err := strconv.ParseFloat(s, 64); err == nil && f > 1e8 {
			s = strconv.FormatFloat(f, 'f', 0, 64)
		}
	}
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) < 8 {
		return out, newErr("invalid_phone", "phone too short after normalization: "+strconv.Quote(raw))
	}
	return out, nil
}

// TransformEmail trims and lowercases. Validation is intentionally weak —
// "x@y.z" is enough; the bot may bounce later anyway.
func TransformEmail(raw string) (string, *TransformError) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return "", nil
	}
	at := strings.Index(s, "@")
	if at < 1 || at == len(s)-1 || !strings.Contains(s[at+1:], ".") {
		return s, newErr("invalid_email", "expected x@y.z, got "+strconv.Quote(raw))
	}
	return s, nil
}

// TransformValue is a generic dispatch for custom fields whose type isn't known
// at compile time. Returns the typed value as `any` plus the error. Used for
// custom field extraction in mapping_parser.
func TransformValue(raw, fieldType string, options []string) (any, *TransformError) {
	switch strings.ToLower(strings.TrimSpace(fieldType)) {
	case "number", "percentage":
		return TransformNumber(raw)
	case "boolean":
		v, err := TransformBool(raw, false)
		return v, err
	case "date":
		t, err := TransformDate(raw)
		if err != nil || t.IsZero() {
			return nil, err
		}
		return t.Format("2006-01-02"), nil
	case "select", "enum":
		return TransformEnum(raw, options)
	case "money", "currency":
		return TransformCurrency(raw)
	case "phone":
		return TransformPhone(raw)
	case "email":
		return TransformEmail(raw)
	default:
		s := strings.TrimSpace(raw)
		if s == "" {
			return nil, nil
		}
		return s, nil
	}
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// stripUnits removes whitespace and common trailing/leading units from a
// number-ish string. It does NOT touch decimal separators.
func stripUnits(s string) string {
	s = strings.TrimSpace(s)
	// Remove anything that's not digit / sign / decimal separator from the
	// edges (e.g., "12,000 IDR" → "12,000").
	for len(s) > 0 {
		r := rune(s[len(s)-1])
		if unicode.IsLetter(r) || unicode.IsSpace(r) {
			s = strings.TrimSpace(s[:len(s)-1])
			continue
		}
		break
	}
	for len(s) > 0 {
		r := rune(s[0])
		if unicode.IsLetter(r) || unicode.IsSpace(r) {
			s = strings.TrimSpace(s[1:])
			continue
		}
		break
	}
	return s
}
