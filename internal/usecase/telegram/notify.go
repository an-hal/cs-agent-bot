package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/rs/zerolog"
)

type TelegramNotifier interface {
	SendMessage(ctx context.Context, chatID string, message string) error
	FormatEscalation(esc entity.Escalation, client entity.Client) string
	FormatPaymentClaim(client entity.Client, replyText string) string
}

type telegramNotifier struct {
	botToken   string
	leadID     string
	httpClient *http.Client
	logger     zerolog.Logger
}

func NewTelegramNotifier(botToken, leadID string, logger zerolog.Logger) TelegramNotifier {
	return &telegramNotifier{
		botToken: botToken,
		leadID:   leadID,
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	t.logger.Info().
		Str("chat_id", chatID).
		Msg("Telegram message sent")

	return nil
}

func (t *telegramNotifier) FormatEscalation(esc entity.Escalation, client entity.Client) string {
	return fmt.Sprintf(
		"<b>ESCALATION %s</b>\n"+
			"Priority: %s\n"+
			"Company: %s (%s)\n"+
			"Reason: %s\n"+
			"Status: %s\n"+
			"Action: Bot suspended. Please resolve in the dashboard.",
		esc.EscID, esc.Priority,
		client.CompanyName, client.CompanyID,
		esc.Reason,
		esc.Status,
	)
}

func (t *telegramNotifier) FormatPaymentClaim(client entity.Client, replyText string) string {
	return fmt.Sprintf(
		"<b>PAYMENT CLAIM</b>\n"+
			"Company: %s (%s)\n"+
			"Client says: %s\n"+
			"Action: Please verify payment proof. Do NOT mark as paid without confirmation.",
		client.CompanyName, client.CompanyID,
		replyText,
	)
}
