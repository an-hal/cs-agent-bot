package analytics

import (
	"math"
	"testing"
)

func TestOLSRegression_KnownDataset(t *testing.T) {
	// y = 2x + 1 with some noise
	xs := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	ys := []float64{1.1, 2.9, 5.2, 6.8, 9.1, 11.0, 12.8, 15.1, 17.0, 19.2}

	slope, intercept, rSquared, residualStd := olsRegression(xs, ys)

	// slope should be close to 2.0
	if math.Abs(slope-2.0) > 0.15 {
		t.Errorf("slope = %f, want ~2.0", slope)
	}

	// intercept should be close to 1.0
	if math.Abs(intercept-1.0) > 0.5 {
		t.Errorf("intercept = %f, want ~1.0", intercept)
	}

	// R-squared should be high (>0.99)
	if rSquared < 0.99 {
		t.Errorf("r_squared = %f, want > 0.99", rSquared)
	}

	// Residual std should be small
	if residualStd > 0.3 {
		t.Errorf("residual_std = %f, want < 0.3", residualStd)
	}
}

func TestOLSRegression_SinglePoint(t *testing.T) {
	slope, intercept, rSquared, residualStd := olsRegression([]float64{0}, []float64{5.0})

	if slope != 0 {
		t.Errorf("slope = %f, want 0", slope)
	}
	if intercept != 5.0 {
		t.Errorf("intercept = %f, want 5.0", intercept)
	}
	if rSquared != 0 {
		t.Errorf("r_squared = %f, want 0", rSquared)
	}
	if residualStd != 0 {
		t.Errorf("residual_std = %f, want 0", residualStd)
	}
}

func TestOLSRegression_Empty(t *testing.T) {
	slope, intercept, _, _ := olsRegression([]float64{}, []float64{})

	if slope != 0 || intercept != 0 {
		t.Errorf("empty: slope=%f, intercept=%f, want 0, 0", slope, intercept)
	}
}

func TestOLSRegression_TwoPoints(t *testing.T) {
	xs := []float64{0, 1}
	ys := []float64{3.0, 5.0}

	slope, intercept, rSquared, _ := olsRegression(xs, ys)

	if math.Abs(slope-2.0) > 0.01 {
		t.Errorf("slope = %f, want 2.0", slope)
	}
	if math.Abs(intercept-3.0) > 0.01 {
		t.Errorf("intercept = %f, want 3.0", intercept)
	}
	if math.Abs(rSquared-1.0) > 0.01 {
		t.Errorf("r_squared = %f, want 1.0", rSquared)
	}
}

func TestFormatMonth(t *testing.T) {
	got := formatMonth(2025, 4)
	want := "Apr '25"
	if got != want {
		t.Errorf("formatMonth(2025, 4) = %q, want %q", got, want)
	}
}

func TestRoundTo(t *testing.T) {
	tests := []struct {
		val      float64
		decimals int
		want     float64
	}{
		{3.14159, 2, 3.14},
		{3.14159, 3, 3.142},
		{100.0, 0, 100},
		{0.005, 2, 0.01},
	}
	for _, tc := range tests {
		got := roundTo(tc.val, tc.decimals)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("roundTo(%f, %d) = %f, want %f", tc.val, tc.decimals, got, tc.want)
		}
	}
}

func TestForecastExtrapolation(t *testing.T) {
	// Using a known slope/intercept, verify the forecast calculation
	// y = 0.5x + 2.0
	xs := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	ys := make([]float64, 10)
	for i := range xs {
		ys[i] = 0.5*xs[i] + 2.0
	}

	slope, intercept, _, _ := olsRegression(xs, ys)

	// Forecast for x=10 should be ~7.0
	fc := slope*10 + intercept
	if math.Abs(fc-7.0) > 0.01 {
		t.Errorf("forecast at x=10: %f, want ~7.0", fc)
	}

	// Forecast for x=15 should be ~9.5
	fc15 := slope*15 + intercept
	if math.Abs(fc15-9.5) > 0.01 {
		t.Errorf("forecast at x=15: %f, want ~9.5", fc15)
	}
}
