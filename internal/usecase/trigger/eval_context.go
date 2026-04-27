package trigger

import (
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
)

// ClientContext aggregates all data needed for rule condition evaluation.
// Field names used in JSON conditions map to the accessor methods below.
type ClientContext struct {
	Client    entity.Client
	Flags     entity.ClientFlags
	Invoice   *entity.Invoice
	ConvState *entity.ConversationState
}

// GetField returns the value of a named field for condition evaluation.
// Field names match what is stored in the trigger_rules condition JSON.
func (c *ClientContext) GetField(name string) (interface{}, bool) {
	switch name {
	// Client fields
	// TODO: post-CRM-refactor — fields below moved to clients.custom_fields
	// (usage_score, nps_score, segment, quotation_link, rejected,
	// cross_sell_*). Until entity.Client exposes a CustomFields map,
	// these stubs return zero values so trigger rules referencing them
	// cleanly no-op instead of crashing.
	case "usage_score":
		return 0, true
	case "nps_score":
		return 0, true
	case "days_to_expiry":
		return c.Client.DaysToExpiry(), true
	case "days_past_due":
		return c.Client.DaysPastDue(), true
	case "days_since_activation":
		return c.Client.DaysSinceActivation(), true
	case "contract_months":
		return c.Client.ContractMonths, true
	case "segment":
		return "", true
	case "payment_status":
		return c.Client.PaymentStatus, true
	case "response_status":
		return c.Client.ResponseStatus, true
	case "quotation_link":
		return "", true
	case "sequence_cs":
		return c.Client.SequenceCS, true
	case "bot_active":
		return c.Client.BotActive, true
	case "blacklisted":
		return c.Client.Blacklisted, true
	case "rejected":
		return false, true
	case "cross_sell_rejected":
		return false, true
	case "cross_sell_interested":
		return false, true
	case "is_payment_overdue":
		return c.Client.IsPaymentOverdue(), true
	case "activation_date_set":
		return !c.Client.ActivationDate.IsZero(), true

	// Invoice reminder flags (on client)
	case "pre14_sent":
		return c.Client.Pre14Sent, true
	case "pre7_sent":
		return c.Client.Pre7Sent, true
	case "pre3_sent":
		return c.Client.Pre3Sent, true
	case "post1_sent":
		return c.Client.Post1Sent, true
	case "post4_sent":
		return c.Client.Post4Sent, true
	case "post8_sent":
		return c.Client.Post8Sent, true
	case "post15_sent":
		return c.Client.Post15Sent, true

	// Conversation state
	case "conv_should_send":
		if c.ConvState != nil {
			return c.ConvState.ShouldSend(), true
		}
		return true, true

	// Invoice fields
	case "has_active_invoice":
		return c.Invoice != nil, true
	}

	return nil, false
}

// GetFlag returns the value of a named flag from ClientFlags.
func (c *ClientContext) GetFlag(name string) (bool, bool) {
	switch name {
	// Health flags
	case "low_usage_msg_sent":
		return c.Flags.LowUsageMsgSent, true
	case "low_nps_msg_sent":
		return c.Flags.LowNPSMsgSent, true

	// Renewal flags
	case "ren60_sent":
		return c.Flags.Ren60Sent, true
	case "ren45_sent":
		return c.Flags.Ren45Sent, true
	case "ren30_sent":
		return c.Flags.Ren30Sent, true
	case "ren15_sent":
		return c.Flags.Ren15Sent, true
	case "ren0_sent":
		return c.Flags.Ren0Sent, true

	// Check-in Branch A
	case "checkin_a1_form_sent":
		return c.Flags.CheckinA1FormSent, true
	case "checkin_a1_call_sent":
		return c.Flags.CheckinA1CallSent, true
	case "checkin_a2_form_sent":
		return c.Flags.CheckinA2FormSent, true
	case "checkin_a2_call_sent":
		return c.Flags.CheckinA2CallSent, true

	// Check-in Branch B
	case "checkin_b1_form_sent":
		return c.Flags.CheckinB1FormSent, true
	case "checkin_b1_call_sent":
		return c.Flags.CheckinB1CallSent, true
	case "checkin_b2_form_sent":
		return c.Flags.CheckinB2FormSent, true
	case "checkin_b2_call_sent":
		return c.Flags.CheckinB2CallSent, true

	// Check-in replied
	case "checkin_replied":
		return c.Flags.CheckinReplied, true

	// NPS + Referral
	case "nps1_sent":
		return c.Flags.NPS1Sent, true
	case "nps2_sent":
		return c.Flags.NPS2Sent, true
	case "nps3_sent":
		return c.Flags.NPS3Sent, true
	case "nps_replied":
		return c.Flags.NPSReplied, true
	case "referral_sent_this_cycle":
		return c.Flags.ReferralSentThisCycle, true

	// Cross-sell 90-day
	case "cs_h7":
		return c.Flags.CSH7, true
	case "cs_h14":
		return c.Flags.CSH14, true
	case "cs_h21":
		return c.Flags.CSH21, true
	case "cs_h30":
		return c.Flags.CSH30, true
	case "cs_h45":
		return c.Flags.CSH45, true
	case "cs_h60":
		return c.Flags.CSH60, true
	case "cs_h75":
		return c.Flags.CSH75, true
	case "cs_h90":
		return c.Flags.CSH90, true

	// Cross-sell long-term
	case "cs_lt1":
		return c.Flags.CSLT1, true
	case "cs_lt2":
		return c.Flags.CSLT2, true
	case "cs_lt3":
		return c.Flags.CSLT3, true

	// Feature update
	case "feature_update_sent":
		return c.Flags.FeatureUpdateSent, true
	case "quotation_acknowledged":
		return c.Flags.QuotationAcknowledged, true
	}

	return false, false
}
