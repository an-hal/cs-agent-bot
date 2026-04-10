package entity

import "time"

type SystemConfig struct {
	Key         string     `json:"key"`
	Value       string     `json:"value"`
	Description string     `json:"description,omitempty"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
	UpdatedBy   string     `json:"updated_by,omitempty"`
}

// System config key constants
const (
	// HaloAI / WhatsApp
	ConfigHaloAIAPIURL     = "HALOAI_API_URL"
	ConfigWAAPIToken       = "WA_API_TOKEN"
	ConfigHaloAIBusinessID = "HALOAI_BUSINESS_ID"
	ConfigHaloAIChannelID  = "HALOAI_CHANNEL_ID"

	// Telegram
	ConfigTelegramBotToken = "TELEGRAM_BOT_TOKEN"
	ConfigTelegramAELeadID = "TELEGRAM_AE_LEAD_ID"

	// Business config
	ConfigPromoDeadline        = "PROMO_DEADLINE"
	ConfigSurveyPlatformURL    = "SURVEY_PLATFORM_URL"
	ConfigCheckinFormURL       = "CHECKIN_FORM_URL"
	ConfigReferralBenefit      = "REFERRAL_BENEFIT"
	ConfigQuotationURL         = "QUOTATION_URL"
	ConfigACVHighThreshold     = "ACV_HIGH_THRESHOLD"
	ConfigACVMidThreshold      = "ACV_MID_THRESHOLD"
	ConfigSeniorAETelegramIDs  = "SENIOR_AE_TELEGRAM_IDS"
	ConfigAETeamTelegramIDs    = "AE_TEAM_TELEGRAM_IDS"
	ConfigAngryKeywordsExtra   = "ANGRY_KEYWORDS_EXTRA"
	ConfigSilenceThresholdDays = "SILENCE_THRESHOLD_DAYS"

	// Auth
	ConfigJWTValidateURL = "JWT_VALIDATE_URL"
)
