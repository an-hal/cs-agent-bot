package collection

import (
	"fmt"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

// parsedFilter is the SQL fragment and args assembled for the record list query.
type parsedFilter struct {
	sql  string
	args []any
}

// parseFilterDSL translates a subset of the project-wide filter DSL into a SQL
// fragment addressing `collection_records.data`. It supports the predicates the
// UI dropdowns emit (§03-api-endpoints.md): `in`, `=`, `prefix`. Unknown keys
// are rejected — the caller must provide the collection's field schema so we
// can guard against SQL injection via an unrestricted identifier.
//
// DSL grammar (whitespace flexible, comma-separated AND'd expressions):
//
//	data.<key> in [<v1>,<v2>,...]
//	data.<key> = <v>
//	data.<key> prefix <v>
//	data.<key> != <v>
//
// Example:
//
//	data.category in ["A","B"], data.created_on prefix "2026-04-15"
//
// Returns empty parsedFilter on empty input.
func parseFilterDSL(filter string, fields []entity.CollectionField, startingArg int) (parsedFilter, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return parsedFilter{}, nil
	}

	keys := make(map[string]entity.CollectionField, len(fields))
	for _, f := range fields {
		keys[f.Key] = f
	}

	parts := splitTopLevel(filter, ',')
	var clauses []string
	var args []any
	argN := startingArg

	for _, part := range parts {
		raw := strings.TrimSpace(part)
		if raw == "" {
			continue
		}
		// Recognise "data.<key>" prefix.
		if !strings.HasPrefix(raw, "data.") {
			return parsedFilter{}, apperror.ValidationError("filter clause must start with data.<key>: " + raw)
		}
		raw = strings.TrimPrefix(raw, "data.")
		// Split on first whitespace to extract key, rest = "<op> <value>".
		sp := strings.IndexAny(raw, " \t")
		if sp < 0 {
			return parsedFilter{}, apperror.ValidationError("filter clause missing operator: " + part)
		}
		key := strings.TrimSpace(raw[:sp])
		rest := strings.TrimSpace(raw[sp+1:])
		if _, ok := keys[key]; !ok {
			return parsedFilter{}, apperror.ValidationError("filter references unknown field: " + key)
		}

		lower := strings.ToLower(rest)
		switch {
		case strings.HasPrefix(lower, "in "):
			vals, err := parseListLiteral(rest[3:])
			if err != nil {
				return parsedFilter{}, err
			}
			if len(vals) == 0 {
				return parsedFilter{}, apperror.ValidationError("empty in() list for " + key)
			}
			phs := make([]string, 0, len(vals))
			for _, v := range vals {
				args = append(args, v)
				argN++
				phs = append(phs, fmt.Sprintf("$%d", argN))
			}
			clauses = append(clauses, fmt.Sprintf("(data->>'%s') IN (%s)", key, strings.Join(phs, ",")))

		case strings.HasPrefix(lower, "prefix "):
			v := unquote(strings.TrimSpace(rest[len("prefix "):]))
			args = append(args, v+"%")
			argN++
			clauses = append(clauses, fmt.Sprintf("(data->>'%s') LIKE $%d", key, argN))

		case strings.HasPrefix(rest, "!="):
			v := unquote(strings.TrimSpace(rest[2:]))
			args = append(args, v)
			argN++
			clauses = append(clauses, fmt.Sprintf("(data->>'%s') <> $%d", key, argN))

		case strings.HasPrefix(rest, "="):
			v := unquote(strings.TrimSpace(rest[1:]))
			args = append(args, v)
			argN++
			clauses = append(clauses, fmt.Sprintf("(data->>'%s') = $%d", key, argN))

		default:
			return parsedFilter{}, apperror.ValidationError("unsupported filter operator in: " + part)
		}
	}

	if len(clauses) == 0 {
		return parsedFilter{}, nil
	}
	return parsedFilter{
		sql:  strings.Join(clauses, " AND "),
		args: args,
	}, nil
}

// parseListLiteral parses `["a","b","c"]` or `[a,b,c]` into string slices.
func parseListLiteral(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, apperror.ValidationError("list literal must be bracketed: " + s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil, nil
	}
	parts := splitTopLevel(inner, ',')
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, unquote(strings.TrimSpace(p)))
	}
	return out, nil
}

// splitTopLevel splits s on sep, ignoring occurrences inside "..." or [...].
func splitTopLevel(s string, sep rune) []string {
	var parts []string
	var buf strings.Builder
	depth := 0
	inStr := false
	for _, r := range s {
		switch {
		case r == '"':
			inStr = !inStr
			buf.WriteRune(r)
		case r == '[' && !inStr:
			depth++
			buf.WriteRune(r)
		case r == ']' && !inStr:
			depth--
			buf.WriteRune(r)
		case r == sep && depth == 0 && !inStr:
			parts = append(parts, buf.String())
			buf.Reset()
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	return parts
}

// unquote strips surrounding "..." if present.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
