package haloai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

type HaloAIClient interface {
	SendWA(ctx context.Context, phoneNumber string, message string) (messageID string, err error)
}

type haloAIClient struct {
	apiURL     string
	apiToken   string
	businessID string
	channelID  string
	httpClient *http.Client
	logger     zerolog.Logger
}

func NewHaloAIClient(apiURL, apiToken, businessID, channelID string, logger zerolog.Logger) HaloAIClient {
	return &haloAIClient{
		apiURL:     apiURL,
		apiToken:   apiToken,
		businessID: businessID,
		channelID:  channelID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

type sendRequest struct {
	ActivateAIAfterSend bool   `json:"activate_ai_after_send"`
	ChannelID           string `json:"channel_id"`
	FallbackTemplate    string `json:"fallback_template_message"`
	PhoneNumber         string `json:"phone_number"`
	Text                string `json:"text"`
}

type sendResponse struct {
	DeliveryStatus string `json:"delivery_status"`
	Error          string `json:"error"`
	MessageID      string `json:"message_id"`
	RoomID         string `json:"room_id"`
	Status         string `json:"status"`
}

func (c *haloAIClient) SendWA(ctx context.Context, phoneNumber string, message string) (string, error) {
	payload, err := json.Marshal(sendRequest{
		ActivateAIAfterSend: false,
		ChannelID:           c.channelID,
		FallbackTemplate:    "",
		PhoneNumber:         phoneNumber,
		Text:                message,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal send request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+"/api/open/channel/whatsapp/v1/sendMessageByPhoneSync", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("X-HaloAI-Business-Id", c.businessID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send WA message: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("HaloAI API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sendResponse
	c.logger.Info().Interface("result", result).Msg("Result")
	c.logger.Info().Interface("body", body).Msg("Body")
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.logger.Info().
		Str("phone", phoneNumber).
		Str("message_id", result.MessageID).
		Msg("WA message sent via HaloAI")

	return result.MessageID, nil
}
