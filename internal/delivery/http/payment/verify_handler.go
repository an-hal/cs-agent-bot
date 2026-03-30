package payment

import (
	"encoding/json"
	"net/http"

	"github.com/Sejutacita/cs-agent-bot/internal/delivery/response"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/apperror"
	usecasePayment "github.com/Sejutacita/cs-agent-bot/internal/usecase/payment"
	"github.com/rs/zerolog"
)

type VerifyPaymentHTTPHandler struct {
	verifier usecasePayment.PaymentVerifier
	logger   zerolog.Logger
}

func NewVerifyPaymentHTTPHandler(verifier usecasePayment.PaymentVerifier, logger zerolog.Logger) *VerifyPaymentHTTPHandler {
	return &VerifyPaymentHTTPHandler{
		verifier: verifier,
		logger:   logger,
	}
}

// VerifyPaymentRequest represents the request payload for payment verification
type VerifyPaymentRequest struct {
	CompanyID  string `json:"company_id"`
	VerifiedBy string `json:"verified_by"`
	InvoiceID  string `json:"invoice_id,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

// Handle godoc
// @Summary      Verify Client Payment
// @Description  Verifies and marks a client's payment as paid after AE verification. Sends confirmation to client via WhatsApp and creates tracking record. Requires HMAC authentication using X-Verify-Secret header.
// @Tags         api
// @Security     X-Verify-Secret
// @Param        request body VerifyPaymentRequest true "Payment verification payload"
// @Success      200  {object}  response.StandardResponse  "Payment verified successfully"
// @Failure      400  {object}  response.StandardResponse  "Invalid request body or validation error"
// @Failure      401  {object}  response.StandardResponse  "Unauthorized - invalid HMAC signature"
// @Failure      404  {object}  response.StandardResponse  "Client not found"
// @Failure      500  {object}  response.StandardResponse  "Internal server error"
// @Router       /api/payment/verify [post]
func (h *VerifyPaymentHTTPHandler) Handle(w http.ResponseWriter, r *http.Request) error {
	var payload VerifyPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return apperror.BadRequest("Invalid request body")
	}

	if payload.CompanyID == "" || payload.VerifiedBy == "" {
		return apperror.ValidationError("company_id and verified_by are required")
	}

	// Convert DTO to usecase type
	ucPayload := usecasePayment.VerifyPaymentRequest{
		CompanyID:  payload.CompanyID,
		VerifiedBy: payload.VerifiedBy,
		InvoiceID:  payload.InvoiceID,
		Notes:      payload.Notes,
	}

	if err := h.verifier.VerifyPayment(r.Context(), ucPayload); err != nil {
		h.logger.Error().Err(err).Str("company_id", payload.CompanyID).Msg("Payment verification failed")
		return err
	}

	return response.StandardSuccess(w, r, http.StatusOK, "Payment verified successfully", nil)
}
