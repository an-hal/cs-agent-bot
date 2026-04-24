package firefliesclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/usecase/mockoutbox"
	"github.com/rs/zerolog"
)

// MockConfig tunes the mock fetch.
type MockConfig struct {
	BaseLatencyMS int
	Outbox        *mockoutbox.Outbox
}

// NewMockClient returns a Client that fabricates a plausible transcript from
// the fireflies_id (deterministic). Use when FIREFLIES_API_KEY is absent or
// MOCK_EXTERNAL_APIS=true.
func NewMockClient(cfg MockConfig, logger zerolog.Logger) Client {
	if cfg.BaseLatencyMS <= 0 {
		cfg.BaseLatencyMS = 300
	}
	return &mockClient{cfg: cfg, logger: logger}
}

type mockClient struct {
	cfg    MockConfig
	logger zerolog.Logger
}

func (c *mockClient) FetchTranscript(ctx context.Context, firefliesID string) (*Transcript, error) {
	time.Sleep(time.Duration(c.cfg.BaseLatencyMS) * time.Millisecond)

	if strings.TrimSpace(firefliesID) == "" {
		if c.cfg.Outbox != nil {
			c.cfg.Outbox.Record(
				mockoutbox.ProviderFireflies, "fetch_transcript",
				map[string]any{"fireflies_id": firefliesID},
				nil, "failed", "fireflies_id required",
			)
		}
		return nil, fmt.Errorf("fireflies_id required")
	}

	t := synthesizeTranscript(firefliesID)

	if c.cfg.Outbox != nil {
		c.cfg.Outbox.Record(
			mockoutbox.ProviderFireflies, "fetch_transcript",
			map[string]any{"fireflies_id": firefliesID},
			map[string]any{
				"title":            t.Title,
				"text_chars":       len(t.Text),
				"participants":     t.Participants,
				"duration_seconds": t.DurationSeconds,
				"host_email":       t.HostEmail,
			},
			"success", "",
		)
	}
	return t, nil
}

// synthesizeTranscript builds a realistic canned transcript keyed by
// fireflies_id so the same id always yields the same text (deterministic).
func synthesizeTranscript(firefliesID string) *Transcript {
	scenario := pickScenario(firefliesID)

	return &Transcript{
		ID:              firefliesID,
		Title:           scenario.title,
		Text:            scenario.text,
		Participants:    scenario.participants,
		DurationSeconds: scenario.duration,
		HostEmail:       scenario.hostEmail,
	}
}

type mockScenario struct {
	title        string
	text         string
	participants []string
	duration     int
	hostEmail    string
}

// pickScenario uses id hash to choose one of a few canned scenarios.
func pickScenario(id string) mockScenario {
	sum := 0
	for _, b := range []byte(id) {
		sum += int(b)
	}
	switch sum % 4 {
	case 0:
		return mockScenario{
			title: "Discovery — Acme Corp",
			text: `BD: Hi Pak, terima kasih waktunya. Boleh cerita dulu kondisi HR di Acme?
Client: Kami sekitar 250 karyawan. Payroll masih pakai spreadsheet, sering telat di akhir bulan.
BD: Siapa yang handle payroll sekarang?
Client: HR Manager kami, tapi saya sebagai CFO yang approve. Saya yang putuskan soal anggaran HR tools.
BD: Anggarannya untuk tahun ini sudah ada?
Client: Sudah, sekitar 150 juta untuk HRIS tahun ini. Penting banget, timing urgent karena audit tahunan.
BD: Bagus. Kapan kira-kira bisa review proposal?
Client: Minggu ini saja kalau bisa, saya tertarik.`,
			participants: []string{"bd@kantorku.id", "cfo@acme.co.id", "hrmanager@acme.co.id"},
			duration:     1500,
			hostEmail:    "bd@kantorku.id",
		}
	case 1:
		return mockScenario{
			title: "Follow-up — Beta Industries",
			text: `BD: Pak, ingin follow up dari email kemarin soal HRIS.
Client: Iya, saya sudah baca. Tapi kami tahun depan saja nanti, budget belum approve.
BD: Untuk scope, berapa karyawan Beta sekarang?
Client: Sekitar 80 orang. Tidak terlalu urgent sih, attendance masih manual tapi ok.
BD: Siapa yang putuskan untuk tools seperti ini?
Client: Harus approval dari direksi, saya cuma HR Manager.
BD: Boleh kami kirim case study dulu?
Client: Boleh, tapi ga janji decision cepat ya.`,
			participants: []string{"bd@kantorku.id", "hrmanager@beta.id"},
			duration:     900,
			hostEmail:    "bd@kantorku.id",
		}
	case 2:
		return mockScenario{
			title: "Demo Request — Gamma Enterprise",
			text: `CEO: Saya langsung to the point. Kami 1000+ karyawan, butuh HRIS enterprise.
BD: Tentu Pak, boleh share pain point utama?
CEO: Payroll accuracy, leave management, dan reporting untuk audit. Sekarang pakai Talenta tapi banyak gap.
BD: Untuk decision timeline?
CEO: Q2 tahun ini harus GO. Budget 2 milyar sudah earmark.
BD: Siapa stakeholder yang akan di-loop in?
CEO: Saya CEO langsung decide. CFO review saja.
BD: Mau schedule demo full team minggu depan?
CEO: Demo sekarang kalau bisa, saya punya 20 menit.`,
			participants: []string{"bd@kantorku.id", "ceo@gamma.co.id"},
			duration:     1200,
			hostEmail:    "bd@kantorku.id",
		}
	}
	return mockScenario{
		title: "Intro call — Delta Startup",
		text: `BD: Halo mas, terima kasih waktunya. Boleh kenalan dulu?
Client: Saya HR generalist di Delta, 50 orang doang.
BD: Concern utama sekarang apa?
Client: Spreadsheet drift, absensi ga sync, tapi kecil-kecil aja. Tidak terlalu penting.
BD: Budget untuk HR tools ada?
Client: Belum, nanti saya tanya bos dulu.
BD: Ok, saya follow up minggu depan ya.
Client: Nanti saja mas, tahun depan aja barangkali.`,
		participants: []string{"bd@kantorku.id", "hr@delta.id"},
		duration:     600,
		hostEmail:    "bd@kantorku.id",
	}
}
