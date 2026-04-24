package dashboard

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/http/router"
	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	claudeextraction "github.com/Sejutacita/cs-agent-bot/internal/usecase/claude_extraction"
	firefliesclient "github.com/Sejutacita/cs-agent-bot/internal/usecase/fireflies_client"
	haloaimock "github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai_mock"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/mockoutbox"
	smtpclient "github.com/Sejutacita/cs-agent-bot/internal/usecase/smtp_client"
	"github.com/rs/zerolog"
)

// MockHandler exposes the mock outbox + per-provider trigger endpoints. These
// are intended for FE QA / integration tests — they never touch real external
// APIs.
type MockHandler struct {
	outbox       *mockoutbox.Outbox
	claudeClient claudeextraction.Client
	ffClient     firefliesclient.Client
	waSender     haloaimock.Sender
	smtpClient   smtpclient.Client
	logger       zerolog.Logger
	tracer       tracer.Tracer
}

// NewMockHandler constructs the handler. Any client may be nil — the matching
// POST endpoint returns 404 in that case so FE can detect what's wired.
func NewMockHandler(
	outbox *mockoutbox.Outbox,
	claudeClient claudeextraction.Client,
	ffClient firefliesclient.Client,
	waSender haloaimock.Sender,
	smtpClient smtpclient.Client,
	logger zerolog.Logger,
	tr tracer.Tracer,
) *MockHandler {
	return &MockHandler{
		outbox:       outbox,
		claudeClient: claudeClient,
		ffClient:     ffClient,
		waSender:     waSender,
		smtpClient:   smtpClient,
		logger:       logger,
		tracer:       tr,
	}
}

// ─── Outbox views ───────────────────────────────────────────────────────────

// ListOutbox godoc
// @Summary  List mock external-API call records
// @Tags     Mock
// @Param    provider  query  string  false  "claude|fireflies|haloai|smtp (empty = all)"
// @Param    limit     query  int     false  "Max records (default 50)"
// @Router   /mock/outbox [get]
func (h *MockHandler) ListOutbox(w http.ResponseWriter, r *http.Request) error {
	if h.outbox == nil {
		return apperror.NotFound("mock_outbox", "mock outbox not wired")
	}
	provider := r.URL.Query().Get("provider")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Mock outbox", map[string]any{
		"records": h.outbox.List(provider, limit),
		"stats":   h.outbox.Stats(),
	})
}

// GetOutboxRecord godoc
// @Summary  Get a single mock outbox record by id
// @Tags     Mock
// @Param    id  path  string  true  "Outbox record id"
// @Router   /mock/outbox/{id} [get]
func (h *MockHandler) GetOutboxRecord(w http.ResponseWriter, r *http.Request) error {
	if h.outbox == nil {
		return apperror.NotFound("mock_outbox", "")
	}
	id := router.GetParam(r, "id")
	rec, ok := h.outbox.Get(id)
	if !ok {
		return apperror.NotFound("mock_outbox_record", id)
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Mock outbox record", rec)
}

// ClearOutbox godoc
// @Summary  Clear the mock outbox (all or by provider)
// @Tags     Mock
// @Param    provider  query  string  false  "Provider to clear (empty = all)"
// @Router   /mock/outbox [delete]
func (h *MockHandler) ClearOutbox(w http.ResponseWriter, r *http.Request) error {
	if h.outbox == nil {
		return apperror.NotFound("mock_outbox", "")
	}
	n := h.outbox.Clear(r.URL.Query().Get("provider"))
	return response.StandardSuccess(w, r, http.StatusOK, "Mock outbox cleared", map[string]int{"cleared": n})
}

// ─── Provider triggers ──────────────────────────────────────────────────────

type mockClaudeReq struct {
	TranscriptText string         `json:"transcript_text"`
	Hints          map[string]any `json:"hints"`
}

// TriggerClaude godoc
// @Summary  Run the Claude mock extractor against an arbitrary transcript
// @Tags     Mock
// @Accept   json
// @Param    body  body  mockClaudeReq  true  "Transcript + hints"
// @Router   /mock/claude/extract [post]
func (h *MockHandler) TriggerClaude(w http.ResponseWriter, r *http.Request) error {
	if h.claudeClient == nil {
		return apperror.NotFound("mock_claude", "mock Claude client not wired")
	}
	var req mockClaudeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if req.TranscriptText == "" {
		return apperror.ValidationError("transcript_text required")
	}
	out, err := h.claudeClient.Extract(r.Context(), req.TranscriptText, req.Hints)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Claude mock extraction", out)
}

type mockFirefliesReq struct {
	FirefliesID string `json:"fireflies_id"`
}

// TriggerFireflies godoc
// @Summary  Fetch a canned Fireflies transcript via the mock client
// @Tags     Mock
// @Accept   json
// @Param    body  body  mockFirefliesReq  true  "Fireflies id"
// @Router   /mock/fireflies/fetch [post]
func (h *MockHandler) TriggerFireflies(w http.ResponseWriter, r *http.Request) error {
	if h.ffClient == nil {
		return apperror.NotFound("mock_fireflies", "mock Fireflies client not wired")
	}
	var req mockFirefliesReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if req.FirefliesID == "" {
		return apperror.ValidationError("fireflies_id required")
	}
	out, err := h.ffClient.FetchTranscript(r.Context(), req.FirefliesID)
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "Fireflies mock transcript", out)
}

type mockHaloAIReq struct {
	WorkspaceID string            `json:"workspace_id"`
	To          string            `json:"to"`
	TemplateID  string            `json:"template_id"`
	Body        string            `json:"body"`
	Variables   map[string]string `json:"variables"`
}

// TriggerHaloAI godoc
// @Summary  Send a WA message through the mock HaloAI sender
// @Tags     Mock
// @Accept   json
// @Param    body  body  mockHaloAIReq  true  "Send request"
// @Router   /mock/haloai/send [post]
func (h *MockHandler) TriggerHaloAI(w http.ResponseWriter, r *http.Request) error {
	if h.waSender == nil {
		return apperror.NotFound("mock_haloai", "mock HaloAI sender not wired")
	}
	var req mockHaloAIReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if req.To == "" {
		return apperror.ValidationError("to required")
	}
	out, err := h.waSender.Send(r.Context(), haloaimock.SendRequest{
		WorkspaceID: req.WorkspaceID,
		To:          req.To,
		TemplateID:  req.TemplateID,
		Body:        req.Body,
		Variables:   req.Variables,
	})
	if err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "HaloAI mock send", out)
}

type mockSMTPReq struct {
	To       []string `json:"to"`
	Cc       []string `json:"cc"`
	Bcc      []string `json:"bcc"`
	Subject  string   `json:"subject"`
	BodyHTML string   `json:"body_html"`
	BodyText string   `json:"body_text"`
	FromAddr string   `json:"from_addr"`
}

// TriggerSMTP godoc
// @Summary  Send an email through the mock SMTP client
// @Tags     Mock
// @Accept   json
// @Param    body  body  mockSMTPReq  true  "Email"
// @Router   /mock/smtp/send [post]
func (h *MockHandler) TriggerSMTP(w http.ResponseWriter, r *http.Request) error {
	if h.smtpClient == nil {
		return apperror.NotFound("mock_smtp", "mock SMTP client not wired")
	}
	var req mockSMTPReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return apperror.BadRequest("invalid JSON body")
	}
	if len(req.To) == 0 {
		return apperror.ValidationError("to required (at least one recipient)")
	}
	if err := h.smtpClient.Send(r.Context(), smtpclient.Message{
		To:       req.To,
		Cc:       req.Cc,
		Bcc:      req.Bcc,
		Subject:  req.Subject,
		BodyHTML: req.BodyHTML,
		BodyText: req.BodyText,
		FromAddr: req.FromAddr,
	}); err != nil {
		return err
	}
	return response.StandardSuccess(w, r, http.StatusOK, "SMTP mock send accepted", map[string]string{"status": "queued"})
}
