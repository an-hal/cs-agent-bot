package main

import (
	"github.com/Sejutacita/cs-agent-bot/config"
	claudeclient "github.com/Sejutacita/cs-agent-bot/internal/usecase/claude_client"
	claudeextractionuc "github.com/Sejutacita/cs-agent-bot/internal/usecase/claude_extraction"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/cron"
	firefliesclient "github.com/Sejutacita/cs-agent-bot/internal/usecase/fireflies_client"
	haloaimock "github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai_mock"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/mockoutbox"
	smtpclient "github.com/Sejutacita/cs-agent-bot/internal/usecase/smtp_client"
	"github.com/rs/zerolog"
)

// mockClients holds the in-memory mock outbox plus every client (real or
// mocked) used by usecases and the /mock HTTP surface. Real clients are
// wired when MOCK_EXTERNAL_APIS=false AND the corresponding API key is set.
//
// Two roles per provider:
//   - "real-or-mock": passed into usecases (claude → claudeextractionuc).
//   - "for-handler": always-mock copies for /mock/* endpoints (so FE QA
//     keeps working even in prod).
type mockClients struct {
	outbox *mockoutbox.Outbox

	claude              claudeextractionuc.Client
	waSenderForCron     cron.WASender
	claudeForHandler    claudeextractionuc.Client
	firefliesForHandler firefliesclient.Client
	waForHandler        haloaimock.Sender
	smtpForHandler      smtpclient.Client
}

func buildMockClients(cfg *config.AppConfig, logger zerolog.Logger) *mockClients {
	outbox := mockoutbox.New(200)
	useMockClaude := cfg.MockExternalAPIs || cfg.ClaudeAPIKey == ""
	useMockHaloAI := cfg.MockExternalAPIs || cfg.WAAPIToken == ""

	mc := &mockClients{outbox: outbox}

	if useMockClaude {
		mc.claude = claudeclient.NewMockClient(claudeclient.MockConfig{Outbox: outbox}, logger)
	} else {
		mc.claude = claudeclient.NewClient(claudeclient.Config{
			APIKey:              cfg.ClaudeAPIKey,
			Model:               cfg.ClaudeModel,
			ExtractionPromptKey: cfg.ClaudeExtractPrompt,
			BANTSPromptKey:      cfg.ClaudeBANTSPrompt,
			Timeout:             cfg.ClaudeTimeoutSecs,
		}, logger)
	}

	// HaloAI WA sender for the cron dispatcher — only wired when running
	// against the mock; in real mode the cron dispatcher receives nil and
	// skips outbound sends.
	if useMockHaloAI {
		mc.waForHandler = haloaimock.NewSender(outbox, logger)
		mc.waSenderForCron = &mockWASenderAdapter{mock: mc.waForHandler}
	}

	// Always-mock instances for the /mock HTTP surface so FE QA keeps working
	// even when MOCK_EXTERNAL_APIS=false.
	mc.claudeForHandler = claudeclient.NewMockClient(claudeclient.MockConfig{Outbox: outbox}, logger)
	mc.firefliesForHandler = firefliesclient.NewMockClient(firefliesclient.MockConfig{Outbox: outbox}, logger)
	if mc.waForHandler == nil {
		mc.waForHandler = haloaimock.NewSender(outbox, logger)
	}
	mc.smtpForHandler = smtpclient.NewMockClient(smtpclient.MockConfig{Outbox: outbox}, logger)
	return mc
}
