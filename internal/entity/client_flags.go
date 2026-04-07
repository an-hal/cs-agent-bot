package entity

type ClientFlags struct {
	CompanyID string `json:"company_id"`

	// Renewal flags — reset by resetCycleFlags() when Renewed=TRUE
	Ren60Sent bool `json:"ren60_sent"`
	Ren45Sent bool `json:"ren45_sent"`
	Ren30Sent bool `json:"ren30_sent"`
	Ren15Sent bool `json:"ren15_sent"`
	Ren0Sent  bool `json:"ren0_sent"`

	// Check-in Branch A (ContractMonths >= 9)
	CheckinA1FormSent bool `json:"checkin_a1_form_sent"`
	CheckinA1CallSent bool `json:"checkin_a1_call_sent"`
	CheckinA2FormSent bool `json:"checkin_a2_form_sent"`
	CheckinA2CallSent bool `json:"checkin_a2_call_sent"`

	// Check-in Branch B (ContractMonths < 9)
	CheckinB1FormSent bool `json:"checkin_b1_form_sent"`
	CheckinB1CallSent bool `json:"checkin_b1_call_sent"`
	CheckinB2FormSent bool `json:"checkin_b2_form_sent"`
	CheckinB2CallSent bool `json:"checkin_b2_call_sent"`

	// Check-in replied — also on Client, both must be set
	CheckinReplied bool `json:"checkin_replied"`

	// NPS + Referral — reset each cycle
	NPS1Sent             bool `json:"nps1_sent"`
	NPS2Sent             bool `json:"nps2_sent"`
	NPS3Sent             bool `json:"nps3_sent"`
	NPSReplied           bool `json:"nps_replied"`
	ReferralSentThisCycle bool `json:"referral_sent_this_cycle"`

	LowUsageMsgSent bool `json:"low_usage_msg_sent"`
	LowNPSMsgSent   bool `json:"low_nps_msg_sent"`

	// Cross-sell 90-day — NOT reset on renewal, persists across cycles
	CSH7  bool `json:"cs_h7"`
	CSH14 bool `json:"cs_h14"`
	CSH21 bool `json:"cs_h21"`
	CSH30 bool `json:"cs_h30"`
	CSH45 bool `json:"cs_h45"`
	CSH60 bool `json:"cs_h60"`
	CSH75 bool `json:"cs_h75"`
	CSH90 bool `json:"cs_h90"`

	// Cross-sell long-term — reset after CSLT3 (rotation restarts)
	CSLT1 bool `json:"cs_lt1"`
	CSLT2 bool `json:"cs_lt2"`
	CSLT3 bool `json:"cs_lt3"`

	// Feature update tracking
	FeatureUpdateSent     bool   `json:"feature_update_sent"`
	QuotationAcknowledged bool   `json:"quotation_acknowledged"`
	WorkspaceID           string `json:"workspace_id"`
}
