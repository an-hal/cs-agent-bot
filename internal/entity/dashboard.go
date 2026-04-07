package entity

import "time"

// DashboardEscalation represents an escalation entry for the dashboard API.
type DashboardEscalation struct {
	EscID            string `json:"esc_id"`
	CompanyID        string `json:"Company_ID"`
	Priority         string `json:"priority"`
	PriorityLabel    string `json:"priority_label"`
	TriggerCondition string `json:"trigger_condition"`
	WhoNotified      string `json:"who_notified"`
	Status           string `json:"status"`
}

func MapPaymentStatus(s string) string {
	switch s {
	case "Paid":
		return "Lunas"
	case "Pending":
		return "Menunggu"
	case "Overdue":
		return "Terlambat"
	case "Partial":
		return "Belum bayar"
	default:
		return s
	}
}

func MapRiskFlag(segment string) string {
	switch segment {
	case "High":
		return "High"
	case "Mid":
		return "Mid"
	default:
		return "Low"
	}
}

func MapChannel(ch string) string {
	switch ch {
	case "whatsapp":
		return "WA"
	case "telegram":
		return "Telegram"
	default:
		return ch
	}
}

func TimeToISO(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
