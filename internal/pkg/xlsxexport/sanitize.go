package xlsxexport

// SanitizeCell escapes cell values that Excel would interpret as a formula,
// per OWASP "CSV Injection" / "Formula Injection" guidance. Values beginning
// with '=', '+', '-', '@', tab, or carriage return are prefixed with a single
// quote so Excel treats them as literal text.
//
// This is the counterpart to pkg/xlsximport.sanitizeCell. Export wasn't
// previously guarded — added here and applied at every SetCellValue call site.
func SanitizeCell(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}
