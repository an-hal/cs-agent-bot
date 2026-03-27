package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/usecase/template"
	"github.com/rs/zerolog"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type TelegramNotifier interface {
	SendMessage(ctx context.Context, chatID string, message string) error
	FormatEscalation(ctx context.Context, esc entity.Escalation, client entity.Client, extraVars map[string]string) (string, error)
	FormatPaymentClaim(client entity.Client, inv *entity.Invoice) string
}

type telegramNotifier struct {
	botToken         string
	leadID           string
	httpClient       *http.Client
	templateResolver template.TemplateResolver
	logger           zerolog.Logger
}

func NewTelegramNotifier(botToken, leadID string, templateResolver template.TemplateResolver, logger zerolog.Logger) TelegramNotifier {
	return &telegramNotifier{
		botToken:         botToken,
		leadID:           leadID,
		templateResolver: templateResolver,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

type telegramSendRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (t *telegramNotifier) SendMessage(ctx context.Context, chatID string, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	payload, err := json.Marshal(telegramSendRequest{
		ChatID:    chatID,
		Text:      message,
		ParseMode: "HTML",
	})
	if err != nil {
		return fmt.Errorf("failed to marshal telegram request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	t.logger.Info().Msg(string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	t.logger.Info().
		Str("chat_id", chatID).
		Msg("Telegram message sent")

	return nil
}

func (t *telegramNotifier) FormatEscalation(ctx context.Context, esc entity.Escalation, client entity.Client, extraVars map[string]string) (string, error) {
	return t.templateResolver.ResolveEscalationTemplate(ctx, esc.EscID, client, esc, extraVars)
}

func (t *telegramNotifier) FormatPaymentClaim(client entity.Client, inv *entity.Invoice) string {
	return fmt.Sprintf(
		"Halo *%s*\n"+
			"Berikut terdapat pembayaran Invoice dengan detail : \n\n"+
			"Nama Company : *%s*\n"+
			"Jatuh Tempo : %s\n"+
			"Nominal : %s\n"+
			"Status : Paid - Need to Confirm\n\n"+
			"Kami lampirkan bukti trasnfer pada Paper.id. \n"+
			"Silahkan check dashboard Paper.id untuk konfirmasi dan lakukan konfirmasi dengan update status invoice pada dashboard AE kita pada URL : http://biz.kantorku.id/\n\n"+
			"Terimakasih",
		client.OwnerName,
		client.CompanyName,
		inv.DueDate.Format("2 Januari 2006"),
		formatRupiah(inv.Amount),
	)
}

func formatRupiah(amount float64) string {
	p := message.NewPrinter(language.Indonesian)
	return p.Sprintf("Rp %.0f", amount)
}
