package analytics

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

const (
	forecastMonths    = 6
	confidenceMulti   = 1.5
	billionDivisor    = 1_000_000_000.0
)

func (u *analyticsUsecase) RevenueTrend(ctx context.Context, workspaceID string, months int) (*entity.RevenueTrendResponse, error) {
	if months <= 0 {
		months = 16
	}
	actualMonths := months - forecastMonths
	if actualMonths < 2 {
		actualMonths = 2
	}

	snapshots, err := u.snapshotRepo.List(ctx, workspaceID, actualMonths)
	if err != nil {
		return nil, fmt.Errorf("revenue trend snapshots: %w", err)
	}

	targets, err := u.targetRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("revenue trend targets: %w", err)
	}

	targetMap := make(map[string]int64)
	var latestTarget int64
	for _, t := range targets {
		key := fmt.Sprintf("%d-%02d", t.Year, t.Month)
		targetMap[key] = t.TargetAmount
		if t.TargetAmount > latestTarget {
			latestTarget = t.TargetAmount
		}
	}

	// Build x,y arrays for OLS.
	xs := make([]float64, len(snapshots))
	ys := make([]float64, len(snapshots))
	for i, s := range snapshots {
		xs[i] = float64(i)
		ys[i] = float64(s.RevenueActual) / billionDivisor
	}

	slope, intercept, rSquared, residualStd := olsRegression(xs, ys)

	resp := &entity.RevenueTrendResponse{
		Workspace: workspaceID,
		Target:    float64(latestTarget) / billionDivisor,
	}
	resp.Regression.Slope = roundTo(slope, 3)
	resp.Regression.Intercept = roundTo(intercept, 2)
	resp.Regression.RSquared = roundTo(rSquared, 2)
	resp.Regression.ResidualStd = roundTo(residualStd, 2)

	// Actual data points.
	for i, s := range snapshots {
		dp := entity.RevenueDataPoint{
			Month:      formatMonth(s.Year, s.Month),
			Date:       fmt.Sprintf("%d-%02d", s.Year, s.Month),
			IsForecast: false,
		}
		actual := ys[i]
		dp.Actual = &actual

		key := fmt.Sprintf("%d-%02d", s.Year, s.Month)
		if t, ok := targetMap[key]; ok {
			dp.Target = float64(t) / billionDivisor
		} else {
			dp.Target = resp.Target
		}

		resp.Data = append(resp.Data, dp)

		if i == len(snapshots)-1 {
			resp.Summary.LastActual = roundTo(actual, 2)
		}
	}

	// Forecast data points.
	n := len(snapshots)
	var lastForecast float64
	for j := 0; j < forecastMonths; j++ {
		xf := float64(n + j)
		fc := slope*xf + intercept
		fcHigh := fc + confidenceMulti*residualStd
		fcLow := fc - confidenceMulti*residualStd
		if fcLow < 0 {
			fcLow = 0
		}

		t := monthAfter(snapshots, j+1)
		dp := entity.RevenueDataPoint{
			Month:      formatMonth(t.Year(), int(t.Month())),
			Date:       fmt.Sprintf("%d-%02d", t.Year(), int(t.Month())),
			IsForecast: true,
			Target:     resp.Target,
		}
		fcR := roundTo(fc, 2)
		fhR := roundTo(fcHigh, 2)
		flR := roundTo(fcLow, 2)
		dp.Forecast = &fcR
		dp.ForecastHigh = &fhR
		dp.ForecastLow = &flR

		resp.Data = append(resp.Data, dp)
		lastForecast = fc
	}

	resp.Summary.ForecastEnd = roundTo(lastForecast, 2)
	if resp.Summary.LastActual > 0 && len(snapshots) > 1 {
		first := ys[0]
		if first > 0 {
			resp.Summary.GrowthPct = roundTo((resp.Summary.LastActual-first)/first*100, 1)
		}
	}

	if resp.Data == nil {
		resp.Data = []entity.RevenueDataPoint{}
	}

	return resp, nil
}

func (u *analyticsUsecase) ForecastAccuracy(ctx context.Context, workspaceID string) (float64, error) {
	snapshots, err := u.snapshotRepo.List(ctx, workspaceID, 18)
	if err != nil {
		return 0, fmt.Errorf("forecast accuracy snapshots: %w", err)
	}
	if len(snapshots) < 3 {
		return 0, nil
	}

	// Use all-but-last for regression, compare forecast to actual of last month.
	xs := make([]float64, len(snapshots)-1)
	ys := make([]float64, len(snapshots)-1)
	for i := 0; i < len(snapshots)-1; i++ {
		xs[i] = float64(i)
		ys[i] = float64(snapshots[i].RevenueActual) / billionDivisor
	}

	slope, intercept, _, _ := olsRegression(xs, ys)
	predicted := slope*float64(len(xs)) + intercept
	actual := float64(snapshots[len(snapshots)-1].RevenueActual) / billionDivisor

	if actual == 0 {
		return 0, nil
	}

	accuracy := (1 - math.Abs(predicted-actual)/actual) * 100
	if accuracy < 0 {
		accuracy = 0
	}
	return roundTo(accuracy, 1), nil
}

// olsRegression computes ordinary least squares: y = slope*x + intercept.
func olsRegression(xs, ys []float64) (slope, intercept, rSquared, residualStd float64) {
	n := float64(len(xs))
	if n < 2 {
		if len(ys) > 0 {
			return 0, ys[0], 0, 0
		}
		return 0, 0, 0, 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i := range xs {
		sumX += xs[i]
		sumY += ys[i]
		sumXY += xs[i] * ys[i]
		sumX2 += xs[i] * xs[i]
	}

	xBar := sumX / n
	yBar := sumY / n

	denom := sumX2 - n*xBar*xBar
	if denom == 0 {
		return 0, yBar, 0, 0
	}

	slope = (sumXY - n*xBar*yBar) / denom
	intercept = yBar - slope*xBar

	// R-squared.
	var ssRes, ssTot float64
	for i := range xs {
		predicted := slope*xs[i] + intercept
		residual := ys[i] - predicted
		ssRes += residual * residual
		ssTot += (ys[i] - yBar) * (ys[i] - yBar)
	}
	if ssTot > 0 {
		rSquared = 1 - ssRes/ssTot
	}

	// Residual standard deviation.
	if n > 2 {
		residualStd = math.Sqrt(ssRes / (n - 2))
	}

	return slope, intercept, rSquared, residualStd
}

func formatMonth(year, month int) string {
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return t.Format("Jan '06")
}

func monthAfter(snapshots []entity.RevenueSnapshot, offset int) time.Time {
	if len(snapshots) == 0 {
		return time.Now().AddDate(0, offset, 0)
	}
	last := snapshots[len(snapshots)-1]
	t := time.Date(last.Year, time.Month(last.Month), 1, 0, 0, 0, 0, time.UTC)
	return t.AddDate(0, offset, 0)
}

func roundTo(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}
