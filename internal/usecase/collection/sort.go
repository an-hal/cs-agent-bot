package collection

import (
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
)

// parseSort translates `field:asc|desc` into a validated (key, type, desc) triple.
// Supported keys: any declared field key, or the built-ins `created_at` /
// `updated_at`. Values with a `data.` prefix are stripped (spec uses
// `data.title:asc`).
func parseSort(sort string, fields []entity.CollectionField) (key, fieldType string, desc bool, err error) {
	sort = strings.TrimSpace(sort)
	if sort == "" {
		return "created_at", "", true, nil
	}
	parts := strings.Split(sort, ":")
	rawKey := strings.TrimSpace(parts[0])
	dir := "asc"
	if len(parts) > 1 {
		dir = strings.ToLower(strings.TrimSpace(parts[1]))
	}
	if dir != "asc" && dir != "desc" {
		return "", "", false, apperror.ValidationError("sort direction must be asc or desc")
	}

	rawKey = strings.TrimPrefix(rawKey, "data.")
	switch rawKey {
	case "created_at", "updated_at":
		return rawKey, "", dir == "desc", nil
	}

	for _, f := range fields {
		if f.Key == rawKey {
			return rawKey, f.Type, dir == "desc", nil
		}
	}
	return "", "", false, apperror.ValidationError("sort references unknown field: " + rawKey)
}
