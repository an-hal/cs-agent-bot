package reports

import (
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// generateHighlights auto-generates positive and negative highlights based on threshold rules.
func generateHighlights(kpi *entity.KPIData, dist *entity.DistributionData) (positive, negative []string) {
	// Positive highlights.
	if kpi.Revenue.Pct >= 90 {
		positive = append(positive, fmt.Sprintf("Revenue on track: %.1f%% quota attainment", kpi.Revenue.Pct))
	} else if kpi.Revenue.Pct >= 50 {
		positive = append(positive, fmt.Sprintf("Revenue progress: %.1f%% quota attainment", kpi.Revenue.Pct))
	}

	if kpi.NPS.Average >= 8 {
		positive = append(positive, fmt.Sprintf("Strong NPS avg %.1f — %d promoter aktif", kpi.NPS.Average, kpi.NPS.Promoter))
	}

	if dist.Risk.Low > 0 {
		total := dist.Risk.High + dist.Risk.Mid + dist.Risk.Low
		pct := 0
		if total > 0 {
			pct = dist.Risk.Low * 100 / total
		}
		positive = append(positive, fmt.Sprintf("%d klien risiko rendah (%d%% portofolio)", dist.Risk.Low, pct))
	}

	if dist.Engagement.CrossSellInterested > 0 {
		positive = append(positive, fmt.Sprintf("%d peluang cross-sell terbuka", dist.Engagement.CrossSellInterested))
	}

	if dist.Engagement.Renewed > 0 {
		positive = append(positive, fmt.Sprintf("%d klien sudah renewed", dist.Engagement.Renewed))
	}

	// Negative highlights.
	if dist.Risk.High > 0 {
		negative = append(negative, fmt.Sprintf("%d klien high risk perlu perhatian segera", dist.Risk.High))
	}

	expiringOrExpired := dist.ContractExpiry.D0_30
	if expiringOrExpired > 0 {
		negative = append(negative, fmt.Sprintf("%d kontrak expiring < 30 hari", expiringOrExpired))
	}

	if dist.PaymentStatus.Terlambat > 0 {
		negative = append(negative, fmt.Sprintf("%d pembayaran terlambat", dist.PaymentStatus.Terlambat))
	}

	if kpi.NPS.Average > 0 && kpi.NPS.Average < 5 {
		negative = append(negative, fmt.Sprintf("NPS avg rendah: %.1f", kpi.NPS.Average))
	}

	if dist.UsageScore.R0_25 > 0 {
		negative = append(negative, fmt.Sprintf("%d klien dengan usage score < 25", dist.UsageScore.R0_25))
	}

	if positive == nil {
		positive = []string{}
	}
	if negative == nil {
		negative = []string{}
	}

	return positive, negative
}
