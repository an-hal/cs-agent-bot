// Package smtpclient implements email delivery for message templates.
// Configuration is per-workspace (via workspace_integrations, provider='smtp')
// or falls back to a global SMTP env config. Without credentials the client
// logs + returns nil (mail is dropped in dev).
package smtpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"

	"github.com/rs/zerolog"
)

type Config struct {
	Host     string // e.g. smtp.sendgrid.net
	Port     int    // 587
	Username string
	Password string
	FromAddr string // default sender, e.g. noreply@kantorku.id
	UseTLS   bool
}

// Message is the payload to send.
type Message struct {
	To       []string
	Cc       []string
	Bcc      []string
	Subject  string
	BodyHTML string
	BodyText string
	FromAddr string // optional override
}

type Client interface {
	Send(ctx context.Context, m Message) error
}

// NewClient returns a real SMTP client when Host is set, otherwise a noop.
func NewClient(cfg Config, logger zerolog.Logger) Client {
	if cfg.Host == "" {
		logger.Warn().Msg("SMTP client: SMTP_HOST not set — using noop client")
		return &noopClient{logger: logger}
	}
	return &realClient{cfg: cfg, logger: logger}
}

type noopClient struct{ logger zerolog.Logger }

func (c *noopClient) Send(ctx context.Context, m Message) error {
	c.logger.Info().Strs("to", m.To).Str("subject", m.Subject).
		Msg("SMTP noop: dropping outbound email (no config)")
	return nil
}

type realClient struct {
	cfg    Config
	logger zerolog.Logger
}

// Send uses net/smtp with STARTTLS when UseTLS is true. Minimal RFC5322-style
// body assembly — a richer library (go-mail/mail) should replace this when
// attachments or inline images are needed.
func (c *realClient) Send(ctx context.Context, m Message) error {
	if len(m.To) == 0 {
		return fmt.Errorf("smtp: at least one To recipient required")
	}
	from := m.FromAddr
	if from == "" {
		from = c.cfg.FromAddr
	}
	if from == "" {
		return fmt.Errorf("smtp: no From address configured")
	}

	// Build a minimal multipart-ish body. Keep it simple: text OR html (html wins).
	body := m.BodyText
	ct := "text/plain; charset=UTF-8"
	if m.BodyHTML != "" {
		body = m.BodyHTML
		ct = "text/html; charset=UTF-8"
	}
	msg := buildMIME(from, m.To, m.Cc, m.Subject, ct, body)

	addr := fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port)
	var auth smtp.Auth
	if c.cfg.Username != "" {
		auth = smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host)
	}
	rcpts := append(append([]string{}, m.To...), m.Cc...)
	rcpts = append(rcpts, m.Bcc...)

	if c.cfg.UseTLS {
		return sendTLS(addr, c.cfg.Host, auth, from, rcpts, msg)
	}
	return smtp.SendMail(addr, auth, from, rcpts, msg)
}

func buildMIME(from string, to, cc []string, subject, contentType, body string) []byte {
	joined := func(h string, addrs []string) string {
		if len(addrs) == 0 {
			return ""
		}
		out := h + ": "
		for i, a := range addrs {
			if i > 0 {
				out += ", "
			}
			out += a
		}
		return out + "\r\n"
	}
	headers := "From: " + from + "\r\n" +
		joined("To", to) +
		joined("Cc", cc) +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: " + contentType + "\r\n\r\n"
	return []byte(headers + body)
}

// sendTLS opens a TLS connection (STARTTLS) and sends the message.
func sendTLS(addr, host string, auth smtp.Auth, from string, to []string, body []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer c.Close()

	tlsCfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	if err := c.StartTLS(tlsCfg); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	for _, r := range to {
		if err := c.Rcpt(r); err != nil {
			return fmt.Errorf("smtp RCPT: %w", err)
		}
	}
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := wc.Write(body); err != nil {
		_ = wc.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return c.Quit()
}
