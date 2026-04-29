package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// sensitiveConfigKeys are masked (value replaced with "***") in API responses.
var sensitiveConfigKeys = map[string]bool{
	"WA_API_TOKEN":       true,
	"TELEGRAM_BOT_TOKEN": true,
}

type AppConfig struct {
	// Application
	Env         string
	Port        string
	RoutePrefix string
	LogLevel    string

	// PostgreSQL Connection
	DBEnabled  string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// PostgreSQL Pool Settings
	DBMaxOpenConns        int
	DBMaxIdleConns        int
	DBConnMaxLifetime     time.Duration
	DBConnMaxIdleTime     time.Duration
	DBQueryTimeout        time.Duration
	DBStatsLoggingEnabled bool

	// Redis Connection
	RedisEnabled  string
	RedisDB       string
	RedisHost     string
	RedisPort     string
	RedisUsername string
	RedisPassword string

	// HaloAI WhatsApp API
	HaloAIAPIURL     string
	WAAPIToken       string
	HaloAIBusinessID string
	HaloAIChannelID  string
	WAWebhookSecret  string

	// Telegram Bot
	TelegramBotToken string
	TelegramAELeadID string

	// Google Sheets
	GoogleSAKeyFile string
	SpreadsheetID   string

	// GCP Cloud Scheduler OIDC
	AppURL           string
	SchedulerSAEmail string

	// BD Handoff
	HandoffWebhookSecret string

	// Business Config
	PromoDeadline        string
	SurveyPlatformURL    string
	CheckinFormURL       string
	ReferralBenefit      string
	QuotationURL         string
	ACVHighThreshold     float64
	ACVMidThreshold      float64
	SeniorAETelegramIDs  string
	AETeamTelegramIDs    string
	AngryKeywordsExtra   string
	SilenceThresholdDays int


	// Workflow Engine
	UseWorkflowEngine bool

	// Google Calendar (Indonesian holidays for workday checks)
	GoogleCalendarAPIKey string

	// Background Jobs
	ExportStoragePath string

	// OpenTelemetry Tracing
	TracerExporter            string
	TracerZipkinCreateSpanURL string
	TracerServiceName         string
	TracerServiceVersion      string

	// GCP Configuration (auto-detected if empty)
	GCPProject string

	// Error Handling
	EnableStackTrace bool

	// JWT Auth
	JWTValidateURL string

	// JWT Dev Bypass — only respected when Env is "development" or "local".
	// Allows Authorization: Bearer DEV.<email> to skip Sejutacita validation.
	JWTDevBypassEnabled bool

	// Dashboard Auth (Feature 01)
	AuthProxyURL       string
	GoogleClientID     string
	SessionSecret      string

	// Claude (Anthropic) — BD extraction pipeline
	ClaudeAPIKey        string
	ClaudeModel         string
	ClaudeTimeoutSecs   int
	ClaudeExtractPrompt string
	ClaudeBANTSPrompt   string

	// Fireflies (transcript fetch)
	FirefliesAPIKey     string
	FirefliesGraphQLURL string

	// SMTP (email delivery — global fallback if workspace_integrations SMTP not set)
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFromAddr string
	SMTPUseTLS   bool

	// Mock mode — forces noop clients to mock impls that return realistic data
	// and record to the in-memory outbox (viewable at /mock/outbox).
	MockExternalAPIs bool

	// AES-256 key (base64/hex/raw 32 bytes) for workspace_integrations.config
	// secret encryption. Empty = plaintext storage (dev default).
	ConfigEncryptionKey string
}

func LoadConfig() *AppConfig {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env")
	}

	cfg := &AppConfig{
		// Application
		Env:         getEnv("ENV", "development"),
		Port:        getEnv("APP_PORT", "8080"),
		RoutePrefix: getEnv("APP_ROUTE_PREFIX", ""),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		// PostgreSQL Connection
		DBEnabled:             getEnv("DB_ENABLED", "true"),
		DBHost:                getEnv("DB_HOST", "localhost"),
		DBPort:                getEnv("DB_PORT", "5432"),
		DBUser:                getEnv("DB_USER", "postgres"),
		DBPassword:            getEnv("DB_PASSWORD", ""),
		DBName:                getEnv("DB_NAME", "cs_agent_bot"),
		DBSSLMode:             getEnv("DB_SSLMODE", "disable"),
		DBMaxOpenConns:        getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:        getEnvInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime:     getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		DBConnMaxIdleTime:     getEnvDuration("DB_CONN_MAX_IDLE_TIME", 1*time.Minute),
		DBQueryTimeout:        getEnvDuration("DB_QUERY_TIMEOUT", 30*time.Second),
		DBStatsLoggingEnabled: getEnvBool("DB_STATS_LOGGING_ENABLED", false),

		// Redis Connection
		RedisEnabled:  getEnv("REDIS_ENABLED", "true"),
		RedisDB:       getEnv("REDIS_DB", "0"),
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisUsername: getEnv("REDIS_USERNAME", ""),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		// HaloAI WhatsApp API
		HaloAIAPIURL:     getEnv("HALOAI_API_URL", ""),
		WAAPIToken:       getEnv("WA_API_TOKEN", ""),
		HaloAIBusinessID: getEnv("HALOAI_BUSINESS_ID", ""),
		HaloAIChannelID:  getEnv("HALOAI_CHANNEL_ID", ""),
		WAWebhookSecret:  getEnv("WA_WEBHOOK_SECRET", ""),

		// Telegram Bot
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramAELeadID: getEnv("TELEGRAM_AE_LEAD_ID", ""),

		// Google Sheets
		GoogleSAKeyFile: getEnv("GOOGLE_SA_KEY_FILE", "./credentials/service_account.json"),
		SpreadsheetID:   getEnv("SPREADSHEET_ID", ""),

		// GCP Cloud Scheduler OIDC
		AppURL:           getEnv("APP_URL", ""),
		SchedulerSAEmail: getEnv("SCHEDULER_SA_EMAIL", ""),

		// BD Handoff
		HandoffWebhookSecret: getEnv("HANDOFF_WEBHOOK_SECRET", ""),

		// Business Config
		PromoDeadline:        getEnv("PROMO_DEADLINE", ""),
		SurveyPlatformURL:    getEnv("SURVEY_PLATFORM_URL", ""),
		CheckinFormURL:       getEnv("CHECKIN_FORM_URL", ""),
		ReferralBenefit:      getEnv("REFERRAL_BENEFIT", "1 bulan gratis"),
		QuotationURL:         getEnv("QUOTATION_URL", ""),
		ACVHighThreshold:     getEnvFloat("ACV_HIGH_THRESHOLD", 50000000),
		ACVMidThreshold:      getEnvFloat("ACV_MID_THRESHOLD", 5000000),
		SeniorAETelegramIDs:  getEnv("SENIOR_AE_TELEGRAM_IDS", ""),
		AETeamTelegramIDs:    getEnv("AE_TEAM_TELEGRAM_IDS", ""),
		AngryKeywordsExtra:   getEnv("ANGRY_KEYWORDS_EXTRA", ""),
		SilenceThresholdDays: getEnvInt("SILENCE_THRESHOLD_DAYS", 30),


		// Workflow Engine
		UseWorkflowEngine: getEnvBool("USE_WORKFLOW_ENGINE", false),

		// Google Calendar
		GoogleCalendarAPIKey: getEnv("GOOGLE_CALENDAR_API_KEY", ""),

		// Background Jobs
		ExportStoragePath: getEnv("EXPORT_STORAGE_PATH", "./tmp/exports"),

		// OpenTelemetry Tracing
		TracerExporter:            getEnv("TRACER_EXPORTER", "zipkin"),
		TracerZipkinCreateSpanURL: getEnv("TRACER_ZIPKIN_CREATE_SPAN_URL", "http://localhost:9411/api/v2/spans"),
		TracerServiceName:         getEnv("TRACER_SERVICE_NAME", "cs-agent-bot"),
		TracerServiceVersion:      getEnv("TRACER_SERVICE_VERSION", "v1.0.0"),

		// GCP Configuration
		GCPProject: getEnv("GCP_PROJECT", ""),

		// Error Handling
		EnableStackTrace: getEnvBool("ENABLE_STACK_TRACE", false),

		// JWT Auth
		JWTValidateURL:      getEnv("JWT_VALIDATE_URL", "https://api.sejutacita.id/v1/login/self"),
		JWTDevBypassEnabled: getEnvBool("JWT_DEV_BYPASS_ENABLED", false),

		// Dashboard Auth (Feature 01)
		AuthProxyURL:   getEnv("AUTH_PROXY_URL", "https://ms-auth-proxy.up.railway.app"),
		GoogleClientID: getEnv("GOOGLE_CLIENT_ID", ""),
		SessionSecret:  getEnv("SESSION_SECRET", ""),

		// Claude (Anthropic)
		ClaudeAPIKey:        getEnv("ANTHROPIC_API_KEY", ""),
		ClaudeModel:         getEnv("CLAUDE_MODEL", "claude-sonnet-4-6"),
		ClaudeTimeoutSecs:   getEnvInt("CLAUDE_TIMEOUT_SECONDS", 30),
		ClaudeExtractPrompt: getEnv("CLAUDE_EXTRACT_PROMPT", "bd_extract_v1"),
		ClaudeBANTSPrompt:   getEnv("CLAUDE_BANTS_PROMPT", "bants_score_v1"),

		// Fireflies
		FirefliesAPIKey:     getEnv("FIREFLIES_API_KEY", ""),
		FirefliesGraphQLURL: getEnv("FIREFLIES_GRAPHQL_URL", "https://api.fireflies.ai/graphql"),

		// SMTP
		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnvInt("SMTP_PORT", 587),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFromAddr: getEnv("SMTP_FROM_ADDR", ""),
		SMTPUseTLS:   getEnvBool("SMTP_USE_TLS", true),

		// Mock mode (defaults true so local dev works without any external keys).
		MockExternalAPIs: getEnvBool("MOCK_EXTERNAL_APIS", true),

		ConfigEncryptionKey: getEnv("CONFIG_ENCRYPTION_KEY", ""),
	}

	validateRequired(cfg)

	return cfg
}

func validateRequired(cfg *AppConfig) {
	// Only validate keys that must come from env vars (used before DB is available).
	// API keys (HALOAI_API_URL, WA_API_TOKEN, TELEGRAM_*) are validated after DB
	// hydration via ValidateCriticalAfterHydration().
	required := map[string]string{
		"WA_WEBHOOK_SECRET":      cfg.WAWebhookSecret,
		"HANDOFF_WEBHOOK_SECRET": cfg.HandoffWebhookSecret,
		"SESSION_SECRET":         cfg.SessionSecret,
		"GOOGLE_CLIENT_ID":       cfg.GoogleClientID,
	}

	// OIDC config only required for production
	if cfg.Env != "development" && cfg.Env != "local" {
		required["APP_URL"] = cfg.AppURL
		required["SCHEDULER_SA_EMAIL"] = cfg.SchedulerSAEmail
	}

	for key, val := range required {
		if val == "" {
			log.Fatalf("Required environment variable %s is not set", key)
		}
	}
}

// HydrateFromDB overwrites config fields with values from the system_config table.
// A DB value only takes effect when it is non-empty; env-var defaults remain as fallback.
func (c *AppConfig) HydrateFromDB(values map[string]string) {
	if v := values["HALOAI_API_URL"]; v != "" {
		c.HaloAIAPIURL = v
	}
	if v := values["WA_API_TOKEN"]; v != "" {
		c.WAAPIToken = v
	}
	if v := values["HALOAI_BUSINESS_ID"]; v != "" {
		c.HaloAIBusinessID = v
	}
	if v := values["HALOAI_CHANNEL_ID"]; v != "" {
		c.HaloAIChannelID = v
	}
	if v := values["TELEGRAM_BOT_TOKEN"]; v != "" {
		c.TelegramBotToken = v
	}
	if v := values["TELEGRAM_AE_LEAD_ID"]; v != "" {
		c.TelegramAELeadID = v
	}
	if v := values["PROMO_DEADLINE"]; v != "" {
		c.PromoDeadline = v
	}
	if v := values["SURVEY_PLATFORM_URL"]; v != "" {
		c.SurveyPlatformURL = v
	}
	if v := values["CHECKIN_FORM_URL"]; v != "" {
		c.CheckinFormURL = v
	}
	if v := values["REFERRAL_BENEFIT"]; v != "" {
		c.ReferralBenefit = v
	}
	if v := values["QUOTATION_URL"]; v != "" {
		c.QuotationURL = v
	}
	if v := values["ACV_HIGH_THRESHOLD"]; v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.ACVHighThreshold = f
		}
	}
	if v := values["ACV_MID_THRESHOLD"]; v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.ACVMidThreshold = f
		}
	}
	if v := values["SENIOR_AE_TELEGRAM_IDS"]; v != "" {
		c.SeniorAETelegramIDs = v
	}
	if v := values["AE_TEAM_TELEGRAM_IDS"]; v != "" {
		c.AETeamTelegramIDs = v
	}
	if v := values["ANGRY_KEYWORDS_EXTRA"]; v != "" {
		c.AngryKeywordsExtra = v
	}
	if v := values["SILENCE_THRESHOLD_DAYS"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.SilenceThresholdDays = n
		}
	}
	if v := values["JWT_VALIDATE_URL"]; v != "" {
		c.JWTValidateURL = v
	}
}

// ValidateCriticalAfterHydration checks that API keys required for external
// services are set — either from env vars or from the DB via HydrateFromDB.
func (c *AppConfig) ValidateCriticalAfterHydration() {
	critical := map[string]string{
		"HALOAI_API_URL":      c.HaloAIAPIURL,
		"WA_API_TOKEN":        c.WAAPIToken,
		"TELEGRAM_BOT_TOKEN":  c.TelegramBotToken,
		"TELEGRAM_AE_LEAD_ID": c.TelegramAELeadID,
	}
	for key, val := range critical {
		if val == "" {
			log.Fatalf("Critical config %s is not set (set via env var or system_config table)", key)
		}
	}
}

// IsSensitiveConfigKey reports whether the system_config key holds a secret
// that should be masked in API responses.
func IsSensitiveConfigKey(key string) bool {
	return sensitiveConfigKeys[key]
}

func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, exists := os.LookupEnv(key); exists {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(val); err == nil {
			return duration
		}
	}
	return defaultVal
}

// getEnvFloat retrieves a float64 environment variable or returns the default value
func getEnvFloat(key string, defaultVal float64) float64 {
	if val, exists := os.LookupEnv(key); exists {
		if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
			return floatVal
		}
	}
	return defaultVal
}
