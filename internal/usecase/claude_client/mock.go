package claudeclient

import (
	"context"
	"strings"
	"time"

	claudeextraction "github.com/Sejutacita/cs-agent-bot/internal/usecase/claude_extraction"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/mockoutbox"
	"github.com/rs/zerolog"
)

// MockConfig controls the mock client's realism knobs.
type MockConfig struct {
	// BaseLatencyMS simulates API round-trip. Default 400.
	BaseLatencyMS int
	// Outbox, when non-nil, records every Extract call for FE inspection.
	Outbox *mockoutbox.Outbox
}

// NewMockClient returns a deterministic fake that produces plausible BANTS
// scores + extracted fields based on keyword heuristics over the transcript.
// Use when ANTHROPIC_API_KEY is absent or MOCK_EXTERNAL_APIS=true.
func NewMockClient(cfg MockConfig, logger zerolog.Logger) claudeextraction.Client {
	if cfg.BaseLatencyMS <= 0 {
		cfg.BaseLatencyMS = 400
	}
	return &mockClient{cfg: cfg, logger: logger}
}

type mockClient struct {
	cfg    MockConfig
	logger zerolog.Logger
}

func (c *mockClient) Extract(ctx context.Context, transcriptText string, hints map[string]any) (*claudeextraction.Result, error) {
	// Simulate latency so FE can see the "running" state in Fireflies extraction.
	time.Sleep(time.Duration(c.cfg.BaseLatencyMS) * time.Millisecond)

	text := strings.ToLower(transcriptText)
	res := deriveResult(text, hints)

	if c.cfg.Outbox != nil {
		c.cfg.Outbox.Record(
			mockoutbox.ProviderClaude, "extract",
			map[string]any{
				"transcript_chars": len(transcriptText),
				"hints":            hints,
			},
			map[string]any{
				"model":               res.Model,
				"bants_score":         safeFloat(res.BANTSScore),
				"bants_classification": res.BANTSClassification,
				"buying_intent":       res.BuyingIntent,
				"prompt_tokens":       res.PromptTokens,
				"completion_tokens":   res.CompletionTokens,
				"fields_extracted":    len(res.Fields),
			},
			"success", "",
		)
	}
	return res, nil
}

// deriveResult builds a plausible extraction by scanning for BANTS-ish
// keywords. Not meant to be accurate — meant to be varied enough that FE can
// exercise hot/warm/cold rendering, score bars, and field display paths.
func deriveResult(text string, hints map[string]any) *claudeextraction.Result {
	budget := scoreFromKeywords(text, []string{"budget", "anggaran", "rp", "juta", "milyar"}, []string{"belum", "nanti", "ga ada"})
	authority := scoreFromKeywords(text, []string{"ceo", "cfo", "direksi", "saya yang putuskan", "owner"}, []string{"tanya bos", "perlu approval", "konsultasi"})
	need := scoreFromKeywords(text, []string{"butuh", "harus", "penting", "urgent", "prioritas"}, []string{"nanti saja", "tidak perlu"})
	timing := scoreFromKeywords(text, []string{"minggu ini", "bulan ini", "q1", "q2", "sekarang", "segera"}, []string{"tahun depan", "bulan depan", "nanti"})
	sentiment := scoreFromKeywords(text, []string{"bagus", "menarik", "tertarik", "oke", "setuju"}, []string{"mahal", "ribet", "ragu", "keberatan"})

	// Overall = equal-weighted avg × 20 (to get 0-100).
	total := float64(budget+authority+need+timing+sentiment) / 5.0
	score := total * 20

	classification := classify(score)
	intent := intentFromSentiment(sentiment, timing)

	coaching := buildCoaching(budget, authority, need, timing, sentiment)

	// Synthesise a few common discovery fields so FE can render the BD
	// detail panel without a real transcript.
	fields := map[string]any{
		"company_size_estimate":  inferCompanySize(text),
		"primary_pain_point":     inferPainPoint(text),
		"decision_maker_role":    inferDMRole(text),
		"competitor_mentioned":   inferCompetitor(text),
		"next_step":              inferNextStep(text),
		"meeting_sentiment_note": inferSentimentNote(sentiment),
	}
	if hints != nil {
		if title, ok := hints["meeting_title"].(string); ok && title != "" {
			fields["meeting_title"] = title
		}
	}

	return &claudeextraction.Result{
		Fields:              fields,
		Model:               "mock-claude-sonnet-4-6",
		PromptTemplate:      "mock-bd-extract-v1",
		BANTSBudget:         intPtr(budget),
		BANTSAuthority:      intPtr(authority),
		BANTSNeed:           intPtr(need),
		BANTSTiming:         intPtr(timing),
		BANTSSentiment:      intPtr(sentiment),
		BANTSScore:          float64Ptr(score),
		BANTSClassification: classification,
		BuyingIntent:        intent,
		CoachingNotes:       coaching,
		PromptTokens:        len(text)/4 + 300,
		CompletionTokens:    180,
	}
}

// scoreFromKeywords returns a 1–5 signal: baseline 3, +1 per positive hit, -1 per negative.
// Clamped to [1, 5].
func scoreFromKeywords(text string, pos, neg []string) int {
	score := 3
	for _, k := range pos {
		if strings.Contains(text, k) {
			score++
		}
	}
	for _, k := range neg {
		if strings.Contains(text, k) {
			score--
		}
	}
	if score < 1 {
		score = 1
	}
	if score > 5 {
		score = 5
	}
	return score
}

func classify(score float64) string {
	switch {
	case score >= 75:
		return "hot"
	case score >= 50:
		return "warm"
	}
	return "cold"
}

func intentFromSentiment(sentiment, timing int) string {
	avg := (sentiment + timing) / 2
	switch {
	case avg >= 4:
		return "high"
	case avg >= 3:
		return "medium"
	}
	return "low"
}

func buildCoaching(b, a, n, t, s int) string {
	parts := []string{}
	if b <= 2 {
		parts = append(parts, "Budget discussion thin — probe 'how are you currently solving X?' to surface spend.")
	}
	if a <= 2 {
		parts = append(parts, "Decision-maker unclear — ask 'who else signs off?' early next meeting.")
	}
	if n <= 2 {
		parts = append(parts, "Need not compelling — narrow to one acute pain, not a feature tour.")
	}
	if t <= 2 {
		parts = append(parts, "Timing soft — anchor to fiscal-year / annual planning, not 'soon'.")
	}
	if s <= 2 {
		parts = append(parts, "Sentiment cool — consider shorter next touch + relationship-building.")
	}
	if len(parts) == 0 {
		return "Strong qualification signals across BANTS — progress to proposal."
	}
	return strings.Join(parts, " ")
}

func inferCompanySize(text string) string {
	switch {
	case strings.Contains(text, "1000") || strings.Contains(text, "enterprise"):
		return "1000+ HC"
	case strings.Contains(text, "500"):
		return "500-1000 HC"
	case strings.Contains(text, "200") || strings.Contains(text, "mid"):
		return "200-500 HC"
	}
	return "50-200 HC"
}

func inferPainPoint(text string) string {
	switch {
	case strings.Contains(text, "payroll"), strings.Contains(text, "gaji"):
		return "Payroll accuracy + late-month reconciliation"
	case strings.Contains(text, "attendance"), strings.Contains(text, "absensi"):
		return "Attendance tracking + overtime calculation"
	case strings.Contains(text, "leave"), strings.Contains(text, "cuti"):
		return "Leave management + approval workflow"
	case strings.Contains(text, "report"), strings.Contains(text, "laporan"):
		return "Reporting + audit trail for HR ops"
	}
	return "Manual HR workflow + spreadsheet drift"
}

func inferDMRole(text string) string {
	switch {
	case strings.Contains(text, "ceo"):
		return "CEO"
	case strings.Contains(text, "cfo"):
		return "CFO"
	case strings.Contains(text, "hr director"), strings.Contains(text, "head of people"):
		return "HR Director"
	case strings.Contains(text, "hr manager"):
		return "HR Manager"
	}
	return "HR Manager"
}

func inferCompetitor(text string) string {
	competitors := []string{"talenta", "gadjian", "mekari", "darwinbox", "bamboohr"}
	for _, c := range competitors {
		if strings.Contains(text, c) {
			return c
		}
	}
	return ""
}

func inferNextStep(text string) string {
	switch {
	case strings.Contains(text, "demo"):
		return "Schedule product demo"
	case strings.Contains(text, "proposal"), strings.Contains(text, "quotation"):
		return "Send proposal"
	case strings.Contains(text, "pilot"), strings.Contains(text, "trial"):
		return "Set up 30-day pilot"
	}
	return "Follow-up with tailored case study"
}

func inferSentimentNote(sentiment int) string {
	switch sentiment {
	case 5:
		return "Strong positive signals — excited, asking implementation questions."
	case 4:
		return "Leaning positive — open to next step."
	case 3:
		return "Neutral — information-gathering mode."
	case 2:
		return "Skeptical — surfaced objections early."
	}
	return "Cool — minimal engagement signals."
}

func intPtr(v int) *int           { return &v }
func float64Ptr(v float64) *float64 { return &v }
func safeFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
