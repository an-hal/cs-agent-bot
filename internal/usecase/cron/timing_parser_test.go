package cron

import (
	"testing"
	"time"
)

func TestParseTiming_ShortForm(t *testing.T) {
	cases := []struct {
		raw    string
		anchor string
		offset int
	}{
		{"H-90", "contract_end", -90},
		{"H-0", "contract_end", 0},
		{"H0", "contract_end", 0},
		{"H+0", "contract_end", 0},
		{"D+14", "contract_start", 14},
		{"D-3", "invoice_due", -3},
		{"post-activation D+14", "contract_start", 14},
		{"renewal H-30", "contract_end", -30},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			s, err := ParseTiming(c.raw)
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if s.Anchor != c.anchor || s.Offset != c.offset {
				t.Errorf("raw=%q → got anchor=%q offset=%d, want anchor=%q offset=%d",
					c.raw, s.Anchor, s.Offset, c.anchor, c.offset)
			}
		})
	}
}

func TestParseTiming_Indonesian(t *testing.T) {
	cases := []struct {
		raw    string
		anchor string
		offset int
	}{
		{"90 hari sebelum kontrak berakhir", "contract_end", -90},
		{"14 hari setelah aktivasi", "contract_start", 14},
		{"3 hari sebelum jatuh tempo", "invoice_due", -3},
		{"8 hari setelah jatuh tempo", "invoice_due", 8},
		{"30 hari setelah dibayar", "invoice_paid", 30},
		{"7 hari stl interaksi terakhir", "last_interaction", 7},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			s, err := ParseTiming(c.raw)
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if s.Anchor != c.anchor || s.Offset != c.offset {
				t.Errorf("raw=%q → got anchor=%q offset=%d, want anchor=%q offset=%d",
					c.raw, s.Anchor, s.Offset, c.anchor, c.offset)
			}
		})
	}
}

func TestParseTiming_Malformed(t *testing.T) {
	cases := []string{"totally wrong", "H-abc", "D-", "15 hari seblum something"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := ParseTiming(c)
			if err == nil {
				t.Errorf("expected error for %q", c)
			}
		})
	}
}

func TestParseTiming_EmptyStringZeroOffset(t *testing.T) {
	s, err := ParseTiming("")
	if err != nil {
		t.Fatalf("empty should not error: %v", err)
	}
	if s.Offset != 0 {
		t.Errorf("expected zero offset, got %d", s.Offset)
	}
}

func TestMatchesDay_ByDayOffset(t *testing.T) {
	anchor := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// H-90 → day = 2026-03-03
	s, _ := ParseTiming("H-90")
	today := time.Date(2026, 3, 3, 8, 0, 0, 0, time.UTC) // different hour, same day
	if !s.MatchesDay(anchor, today) {
		t.Error("expected H-90 to match 2026-03-03")
	}
	wrongDay := time.Date(2026, 3, 4, 8, 0, 0, 0, time.UTC)
	if s.MatchesDay(anchor, wrongDay) {
		t.Error("H-90 should not match 2026-03-04")
	}
}
