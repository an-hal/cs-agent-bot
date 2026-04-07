package payment

import (
	"context"
	"fmt"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/haloai"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/telegram"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/template"
	"github.com/rs/zerolog"
)

type PaymentVerifier interface {
	VerifyPayment(ctx context.Context, req VerifyPaymentRequest) error
}

type VerifyPaymentRequest struct {
	CompanyID  string `json:"company_id"`
	VerifiedBy string `json:"verified_by"` // AE name or ID
	InvoiceID  string `json:"invoice_id"`  // Optional
	Notes      string `json:"notes"`       // Optional verification notes
}

type paymentVerifier struct {
	clientRepo       repository.ClientRepository
	flagsRepo        repository.FlagsRepository
	logRepo          repository.LogRepository
	escalationRepo   repository.EscalationRepository
	telegramNotifier telegram.TelegramNotifier
	haloaiClient     haloai.HaloAIClient
	templateResolver template.TemplateResolver
	logger           zerolog.Logger
}

func NewPaymentVerifier(
	clientRepo repository.ClientRepository,
	flagsRepo repository.FlagsRepository,
	logRepo repository.LogRepository,
	escalationRepo repository.EscalationRepository,
	telegramNotifier telegram.TelegramNotifier,
	haloaiClient haloai.HaloAIClient,
	templateResolver template.TemplateResolver,
	logger zerolog.Logger,
) PaymentVerifier {
	return &paymentVerifier{
		clientRepo:       clientRepo,
		flagsRepo:        flagsRepo,
		logRepo:          logRepo,
		escalationRepo:   escalationRepo,
		telegramNotifier: telegramNotifier,
		haloaiClient:     haloaiClient,
		templateResolver: templateResolver,
		logger:           logger,
	}
}

func (v *paymentVerifier) VerifyPayment(ctx context.Context, req VerifyPaymentRequest) error {
	// 1. Get client by CompanyID
	client, err := v.clientRepo.GetByCompanyID(ctx, req.CompanyID)
	if err != nil {
		return fmt.Errorf("client not found: %s: %w", req.CompanyID, err)
	}
	if client == nil {
		return fmt.Errorf("client not found: %s", req.CompanyID)
	}

	// 2. Update PaymentStatus to "Paid"
	client.UpdatePaymentStatus(entity.PaymentStatusPaid)
	if err := v.clientRepo.UpdatePaymentStatus(ctx, req.CompanyID, entity.PaymentStatusPaid); err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}

	// 3. Get invoice for template
	invoice, err := v.clientRepo.GetLatestInvoice(ctx, req.CompanyID)
	if err != nil {
		v.logger.Warn().Err(err).Str("company_id", req.CompanyID).Msg("No invoice found, proceeding with verification")
		invoice = nil
	}

	invoiceID := req.InvoiceID
	if invoiceID == "" && invoice != nil {
		invoiceID = invoice.InvoiceID
	}

	// 4. Send WhatsApp to client using TPL_PAY_VERIF template
	templateCfg := template.TemplateConfig{}

	message, err := v.templateResolver.ResolveTemplate(ctx, "TPL_PAY_VERIF", *client, invoice, templateCfg)
	if err != nil {
		return fmt.Errorf("failed to resolve payment verification template: %w", err)
	}

	if _, err := v.haloaiClient.SendWA(ctx, client.PICWA, message); err != nil {
		return fmt.Errorf("failed to send WhatsApp message: %w", err)
	}

	// 5. Create ESC-007 escalation record (for tracking)
	esc := entity.Escalation{
		CompanyID: req.CompanyID,
		EscID:     entity.EscPaymentClaim,
		Status:    entity.EscalationStatusResolved, // Auto-close as tracking only
		Priority:  entity.EscPriorityP2High,
		Reason:    fmt.Sprintf("Payment verified by %s. Invoice: %s", req.VerifiedBy, invoiceID),
	}

	if err := v.escalationRepo.OpenEscalation(ctx, esc); err != nil {
		return fmt.Errorf("failed to create escalation record: %w", err)
	}

	// 6. Log action to Action Log
	logEntry := entity.ActionLog{
		CompanyID:              req.CompanyID,
		TriggerType:            "PAYMENT_VERIFIED",
		TemplateID:             invoiceID,
		Channel:                entity.ChannelWhatsApp,
		MessageSent:            true,
		ResponseReceived:       false,
		ResponseClassification: "",
		NextActionTriggered:    "PAYMENT_VERIFICATION",
		LogNotes:               fmt.Sprintf("Verified by: %s. Notes: %s", req.VerifiedBy, req.Notes),
	}

	if err := v.logRepo.AppendLog(ctx, logEntry); err != nil {
		return fmt.Errorf("failed to log payment verification: %w", err)
	}

	// 7. Send Telegram confirmation to AE using ESC-007 template
	extraVars := map[string]string{
		"Verified_By": req.VerifiedBy,
		"Invoice_ID":  invoiceID,
	}

	telegramMsg, err := v.telegramNotifier.FormatEscalation(ctx, esc, *client, extraVars)
	if err != nil {
		return fmt.Errorf("failed to format telegram message: %w", err)
	}

	if err := v.telegramNotifier.SendMessage(ctx, client.OwnerTelegramID, telegramMsg); err != nil {
		return fmt.Errorf("failed to send telegram notification: %w", err)
	}

	v.logger.Info().
		Str("company_id", req.CompanyID).
		Str("verified_by", req.VerifiedBy).
		Str("invoice_id", invoiceID).
		Msg("Payment verified successfully")

	return nil
}
