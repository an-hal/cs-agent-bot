package cron

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TimingSpec is the normalized form that cron logic evaluates. A rule fires
// when the elapsed offset from `Anchor` matches `Offset` on the schedule's
// trigger calendar.
type TimingSpec struct {
	// Raw is the original string the admin typed (for round-trip display).
	Raw string
	// Anchor indicates what the offset is measured from.
	//   "contract_end"       H-* / D-* styles around renewal
	//   "contract_start"     post-activation style ("D+14")
	//   "last_interaction"   engagement follow-ups
	//   "invoice_due"        invoice reminders
	//   "invoice_paid"       cross-sell series
	Anchor string
	// Offset is the signed day offset. Negative = before, positive = after.
	// Example: H-90 → -90; D+14 → +14; D+0 → 0.
	Offset int
}

// ParseTiming accepts both the legacy shorthand used throughout the codebase
// and the Indonesian spec format. Returns an error only for clearly malformed
// input; an empty string returns a zero-offset spec anchored on contract_end
// so tests and defaults don't have to branch.
//
// Supported forms (all case-insensitive, whitespace-tolerant):
//
//	"H-90"            → Anchor=contract_end, Offset=-90
//	"H+0" / "H0"      → Anchor=contract_end, Offset=0
//	"D+14"            → Anchor=contract_start, Offset=+14
//	"D-3"             → Anchor=invoice_due,    Offset=-3
//	"H-90 sebelum kontrak berakhir"         → contract_end, -90
//	"14 hari setelah aktivasi"              → contract_start, +14
//	"3 hari sebelum jatuh tempo"            → invoice_due, -3
//	"8 hari setelah jatuh tempo"            → invoice_due, +8
//	"post-activation D+14"                  → contract_start, +14
func ParseTiming(raw string) (TimingSpec, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return TimingSpec{Raw: raw, Anchor: "contract_end", Offset: 0}, nil
	}
	lower := strings.ToLower(trimmed)

	// Indonesian long-form — "<N> hari <sebelum|setelah> <anchor>"
	if m := indonesianRE.FindStringSubmatch(lower); m != nil {
		days, err := strconv.Atoi(m[1])
		if err != nil {
			return TimingSpec{Raw: raw}, err
		}
		sign := 1
		if strings.Contains(m[2], "sebelum") {
			sign = -1
		}
		anchor, ok := indonesianAnchor(m[3])
		if !ok {
			return TimingSpec{Raw: raw}, errors.New("unknown indonesian anchor phrase: " + m[3])
		}
		return TimingSpec{Raw: raw, Anchor: anchor, Offset: sign * days}, nil
	}

	// Short-form — normalize "H-90", "H+0", "H0", "D-14", "D+14",
	// "post-activation D+14" (last token wins).
	// Strip common prefixes (post-activation, pre-activation) for the short form.
	noPrefix := shortFormPrefixRE.ReplaceAllString(lower, "")
	noPrefix = strings.TrimSpace(noPrefix)

	if m := shortFormRE.FindStringSubmatch(noPrefix); m != nil {
		letter := strings.ToUpper(m[1])
		signStr := m[2]
		numStr := m[3]
		n, err := strconv.Atoi(numStr)
		if err != nil {
			return TimingSpec{Raw: raw}, err
		}
		if signStr == "-" {
			n = -n
		}
		anchor := "contract_end"
		if letter == "D" {
			// D-* typically invoice_due in the legacy lexicon; D+* typically
			// post-activation. Admins can override via Indonesian long-form.
			if n < 0 {
				anchor = "invoice_due"
			} else {
				anchor = "contract_start"
			}
		}
		return TimingSpec{Raw: raw, Anchor: anchor, Offset: n}, nil
	}

	return TimingSpec{Raw: raw}, errors.New("unrecognized timing format: " + raw)
}

// MatchesDay reports whether the rule should fire for the given `today`
// relative to the anchor time. Both are compared at day granularity so time-
// of-day drift doesn't cause misfires.
func (s TimingSpec) MatchesDay(anchorTime, today time.Time) bool {
	if anchorTime.IsZero() {
		return false
	}
	a := truncateDay(anchorTime)
	t := truncateDay(today)
	diff := int(t.Sub(a).Hours() / 24)
	return diff == s.Offset
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

var (
	// e.g. "H-90", "D+0", "H0", "d-3"
	shortFormRE = regexp.MustCompile(`^([HhDd])\s*([-+]?)\s*(\d+)$`)
	// Strips "post-activation" / "pre-activation" / leading dashes before the short form.
	shortFormPrefixRE = regexp.MustCompile(`^(post-activation|pre-activation|renewal|engagement)\s*[:\-]?\s*`)
	// "14 hari sebelum kontrak berakhir", "3 hari setelah jatuh tempo", etc.
	indonesianRE = regexp.MustCompile(`^(\d+)\s+hari\s+(sebelum|setelah|sblm|stl)\s+(.+?)\s*$`)
)

func indonesianAnchor(phrase string) (string, bool) {
	p := strings.ToLower(strings.TrimSpace(phrase))
	switch {
	case strings.Contains(p, "kontrak berakhir"), strings.Contains(p, "kontrak habis"), strings.Contains(p, "expiry"):
		return "contract_end", true
	case strings.Contains(p, "aktivasi"), strings.Contains(p, "activation"), strings.Contains(p, "kontrak mulai"):
		return "contract_start", true
	case strings.Contains(p, "jatuh tempo"), strings.Contains(p, "due date"):
		return "invoice_due", true
	case strings.Contains(p, "interaksi"), strings.Contains(p, "last interaction"), strings.Contains(p, "terakhir"):
		return "last_interaction", true
	case strings.Contains(p, "dibayar"), strings.Contains(p, "paid"), strings.Contains(p, "lunas"):
		return "invoice_paid", true
	}
	return "", false
}
