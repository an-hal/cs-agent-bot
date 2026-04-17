package reports

import (
	"strings"
	"testing"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

func newTestKPI() *entity.KPIData {
	kpi := &entity.KPIData{}
	kpi.Revenue.Pct = 64.8
	kpi.Clients.Total = 15
	kpi.Clients.Active = 12
	kpi.NPS.Average = 7.8
	kpi.NPS.Promoter = 6
	kpi.Attention.HighRisk = 2
	kpi.Attention.Expiring30d = 3
	return kpi
}

func newTestDist() *entity.DistributionData {
	d := &entity.DistributionData{}
	d.Risk.High = 2
	d.Risk.Mid = 4
	d.Risk.Low = 9
	d.PaymentStatus.Terlambat = 3
	d.ContractExpiry.D0_30 = 3
	d.UsageScore.R0_25 = 2
	d.Engagement.CrossSellInterested = 4
	d.Engagement.Renewed = 6
	return d
}

func TestHighlights_PositiveRevenueOnTrack(t *testing.T) {
	kpi := newTestKPI()
	kpi.Revenue.Pct = 95.0
	dist := newTestDist()

	pos, _ := generateHighlights(kpi, dist)

	found := false
	for _, h := range pos {
		if strings.Contains(h, "on track") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'on track' highlight when revenue >= 90%")
	}
}

func TestHighlights_PositiveRevenueProgress(t *testing.T) {
	kpi := newTestKPI()
	kpi.Revenue.Pct = 64.8
	dist := newTestDist()

	pos, _ := generateHighlights(kpi, dist)

	found := false
	for _, h := range pos {
		if strings.Contains(h, "progress") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'progress' highlight when revenue between 50-90%")
	}
}

func TestHighlights_NegativeHighRisk(t *testing.T) {
	kpi := newTestKPI()
	dist := newTestDist()
	dist.Risk.High = 5

	_, neg := generateHighlights(kpi, dist)

	found := false
	for _, h := range neg {
		if strings.Contains(h, "high risk") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'high risk' negative highlight")
	}
}

func TestHighlights_NegativePaymentTerlambat(t *testing.T) {
	kpi := newTestKPI()
	dist := newTestDist()

	_, neg := generateHighlights(kpi, dist)

	found := false
	for _, h := range neg {
		if strings.Contains(h, "terlambat") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'terlambat' negative highlight when terlambat > 0")
	}
}

func TestHighlights_NoNegativeWhenAllGood(t *testing.T) {
	kpi := newTestKPI()
	kpi.NPS.Average = 9.0
	dist := &entity.DistributionData{}
	dist.Risk.Low = 15

	_, neg := generateHighlights(kpi, dist)

	if len(neg) != 0 {
		t.Errorf("expected 0 negative highlights when all good, got %d: %v", len(neg), neg)
	}
}

func TestHighlights_EmptySlicesNotNil(t *testing.T) {
	kpi := &entity.KPIData{}
	dist := &entity.DistributionData{}

	pos, neg := generateHighlights(kpi, dist)

	if pos == nil {
		t.Error("positive highlights should not be nil")
	}
	if neg == nil {
		t.Error("negative highlights should not be nil")
	}
}
