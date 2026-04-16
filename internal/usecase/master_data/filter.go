package master_data

import (
	"strconv"
	"strings"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// ParseFilter implements the shared filter DSL described in
// 00-shared/01-filter-dsl.md. It returns an entity.MasterDataFilter — the
// repository layer is responsible for translating into SQL with parameter
// binding. We intentionally do not return raw SQL strings here.
func ParseFilter(filter string) entity.MasterDataFilter {
	out := entity.MasterDataFilter{}
	switch {
	case filter == "" || filter == "all":
		return out
	case filter == "bot_active":
		t := true
		out.BotActive = &t
		return out
	case filter == "risk":
		out.RiskFlag = entity.RiskHigh
		return out
	case strings.HasPrefix(filter, "stage:"):
		vals := strings.Split(strings.TrimPrefix(filter, "stage:"), ",")
		for _, v := range vals {
			v = strings.TrimSpace(v)
			if v != "" {
				out.Stages = append(out.Stages, v)
			}
		}
		return out
	case strings.HasPrefix(filter, "payment:"):
		out.PaymentStatus = strings.TrimPrefix(filter, "payment:")
		return out
	case strings.HasPrefix(filter, "expiry:"):
		days, _ := strconv.Atoi(strings.TrimPrefix(filter, "expiry:"))
		out.ExpiryWithin = days
		return out
	}
	return out
}
