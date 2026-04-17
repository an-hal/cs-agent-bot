package invoice

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	"github.com/rs/zerolog"
)

// PaperIDService integrates with the Paper.id payment gateway.
// If a workspace has no Paper.id credentials configured, Create returns empty
// strings and HandleWebhook is never called.
type PaperIDService interface {
	// Create submits an invoice to Paper.id and returns the hosted URL and ref.
	// Returns ("", "", nil) if Paper.id is not configured for the workspace.
	Create(ctx context.Context, ws entity.Workspace, inv entity.Invoice) (paperIDURL, paperIDRef string, err error)

	// HandleWebhook processes an inbound Paper.id webhook event.
	// It verifies the HMAC-SHA256 signature and updates the invoice status.
	// NOTE: This is the only path that may write payment_status to an invoice.
	HandleWebhook(ctx context.Context, wsID string, sig string, payload PaperIDWebhookPayload) error
}

// PaperIDWebhookPayload represents the minimal fields from a Paper.id webhook.
type PaperIDWebhookPayload struct {
	ExternalID     string         `json:"external_id"`
	Status         string         `json:"status"`
	AmountPaid     int64          `json:"amount_paid"`
	PaymentMethod  string         `json:"payment_method"`
	PaymentChannel string         `json:"payment_channel"`
	PaymentRef     string         `json:"payment_ref"`
	PaidAt         time.Time      `json:"paid_at"`
	Raw            map[string]any `json:"_raw,omitempty"`
}

// NoopPaperIDService is a no-op implementation used when Paper.id is not configured.
type NoopPaperIDService struct{}

func (n *NoopPaperIDService) Create(_ context.Context, _ entity.Workspace, _ entity.Invoice) (string, string, error) {
	return "", "", nil
}
func (n *NoopPaperIDService) HandleWebhook(_ context.Context, _ string, _ string, _ PaperIDWebhookPayload) error {
	return nil
}

// paperIDSvc is the live Paper.id implementation.
type paperIDSvc struct {
	uc     *invoiceUsecase
	logger zerolog.Logger
}

// NewPaperIDService constructs the live Paper.id service.
func NewPaperIDService(uc Usecase) PaperIDService {
	u, ok := uc.(*invoiceUsecase)
	if !ok {
		panic("invoice.NewPaperIDService: expected *invoiceUsecase")
	}
	return &paperIDSvc{uc: u, logger: u.logger}
}

// Create submits an invoice to Paper.id.
// If no Paper.id secret is found in workspace settings, returns ("", "", nil).
func (s *paperIDSvc) Create(ctx context.Context, ws entity.Workspace, inv entity.Invoice) (string, string, error) {
	secret, _ := ws.Settings["paper_id_secret"].(string)
	if secret == "" {
		return "", "", nil
	}

	// Stub: return placeholder values until the Paper.id REST client is implemented.
	// Replace with an actual HTTP call to the Paper.id API.
	paperURL := fmt.Sprintf("https://app.paper.id/invoices/%s", inv.InvoiceID)
	paperRef := fmt.Sprintf("PAPERID-%s", inv.InvoiceID)

	s.logger.Info().
		Str("invoice_id", inv.InvoiceID).
		Str("paper_id_url", paperURL).
		Msg("Paper.id: invoice created (stub)")

	return paperURL, paperRef, nil
}

// HandleWebhook verifies the HMAC-SHA256 signature and processes the payment event.
// NOTE: This is the ONLY path that writes payment_status on an invoice (exception to CLAUDE.md rule).
func (s *paperIDSvc) HandleWebhook(ctx context.Context, wsID string, sig string, payload PaperIDWebhookPayload) error {
	// Resolve workspace to get the Paper.id webhook secret.
	ws, err := s.uc.workspaceRepo.GetByID(ctx, wsID)
	if err != nil || ws == nil {
		return apperror.NotFound("workspace", "")
	}
	secret, _ := ws.Settings["paper_id_webhook_secret"].(string)
	if secret == "" {
		return apperror.BadRequest("Paper.id webhook secret not configured for workspace")
	}

	// Verify HMAC-SHA256 signature.
	rawBody, _ := json.Marshal(payload.Raw)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return apperror.Unauthorized("invalid Paper.id webhook signature")
	}

	// Look up the invoice by external_id (= invoice_id).
	inv, err := s.uc.invoiceRepo.GetByID(ctx, payload.ExternalID)
	if err != nil {
		return err
	}
	if inv == nil {
		return apperror.NotFound("invoice", fmt.Sprintf("external_id=%s", payload.ExternalID))
	}

	oldStatus := inv.PaymentStatus
	now := time.Now().UTC()

	// Update invoice: mark paid.
	fields := map[string]interface{}{
		"payment_status":   entity.PaymentStatusLunas,
		"paid_at":          payload.PaidAt,
		"amount_paid":      float64(payload.AmountPaid),
		"payment_method":   payload.PaymentMethod,
		"updated_at":       now,
	}
	if err := s.uc.invoiceRepo.UpdateFields(ctx, inv.InvoiceID, fields); err != nil {
		return fmt.Errorf("paperid webhook: update invoice: %w", err)
	}

	// Append payment log with raw payload.
	amountPaid := payload.AmountPaid
	_ = s.uc.paymentLogRepo.Append(ctx, entity.PaymentLog{
		WorkspaceID:    wsID,
		InvoiceID:      inv.InvoiceID,
		EventType:      entity.EventPaperIDWebhook,
		AmountPaid:     &amountPaid,
		PaymentMethod:  payload.PaymentMethod,
		PaymentChannel: payload.PaymentChannel,
		PaymentRef:     payload.PaymentRef,
		OldStatus:      oldStatus,
		NewStatus:      entity.PaymentStatusLunas,
		Actor:          "paperid-webhook",
		RawPayload:     payload.Raw,
		Timestamp:      now,
	})

	s.logger.Info().
		Str("invoice_id", inv.InvoiceID).
		Str("old_status", oldStatus).
		Msg("Paper.id webhook: invoice marked as Lunas")

	return nil
}
