package trigger

import (
	"context"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// All trigger use cases follow the same pattern:
// EvalXxx returns (sent bool, err error)
// sent is true if a message was sent
// err is any error that occurred

// HealthUseCase evaluates P0 health risk triggers.
type HealthUseCase interface {
	EvalHealthRisk(ctx context.Context, client *entity.Client) (bool, error)
}

// CheckInUseCase evaluates P0.5 check-in triggers.
type CheckInUseCase interface {
	EvalCheckIn(ctx context.Context, client *entity.Client) (bool, error)
}

// NegotiationUseCase evaluates P1 negotiation triggers.
type NegotiationUseCase interface {
	EvalNegotiation(ctx context.Context, client *entity.Client) (bool, error)
}

// InvoiceUseCase evaluates P2 invoice triggers.
type InvoiceUseCase interface {
	EvalInvoice(ctx context.Context, client *entity.Client) (bool, error)
}

// OverdueUseCase evaluates P3 overdue triggers.
type OverdueUseCase interface {
	EvalOverdue(ctx context.Context, client *entity.Client) (bool, error)
}

// ExpansionUseCase evaluates P4 expansion triggers.
type ExpansionUseCase interface {
	EvalExpansion(ctx context.Context, client *entity.Client) (bool, error)
}

// CrossSellUseCase evaluates P5 cross-sell triggers.
type CrossSellUseCase interface {
	EvalCrossSell(ctx context.Context, client *entity.Client) (bool, error)
}

// EscalationUseCase handles escalation triggers.
type EscalationUseCase interface {
	TriggerEscalation(ctx context.Context, client *entity.Client, escID, priority, condition string) error
}

// TemplateUseCase handles template resolution.
type TemplateUseCase interface {
	ResolveTemplate(ctx context.Context, templateID string, data map[string]interface{}) (string, error)
}

// HaloAIUseCase handles WhatsApp sending.
type HaloAIUseCase interface {
	SendWA(ctx context.Context, to, message string) (string, error)
}

// TelegramUseCase handles Telegram notifications.
type TelegramUseCase interface {
	SendMessage(ctx context.Context, telegramID, message string) error
}

// ReplyClassifierUseCase classifies incoming replies.
type ReplyClassifierUseCase interface {
	ClassifyReply(ctx context.Context, message string) (string, error)
}
