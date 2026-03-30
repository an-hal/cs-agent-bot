package entity

type SystemConfig struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// System config key constants
const (
	ConfigSurveyPlatformURL = "SURVEY_PLATFORM_URL"
	ConfigCheckinFormURL    = "CHECKIN_FORM_URL"
	ConfigReferralBenefit   = "REFERRAL_BENEFIT"
	ConfigTelegramAELeadID  = "TELEGRAM_AE_LEAD_ID"
)
