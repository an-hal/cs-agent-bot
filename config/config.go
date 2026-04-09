package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

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

	// Dynamic Rule Engine
	UseDynamicRules bool

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

		// Dynamic Rule Engine
		UseDynamicRules: getEnvBool("USE_DYNAMIC_RULES", false),

		// Background Jobs
		ExportStoragePath: getEnv("EXPORT_STORAGE_PATH", "/tmp/exports"),

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
		JWTValidateURL: getEnv("JWT_VALIDATE_URL", "https://api.sejutacita.id/v1/login/self"),
	}

	validateRequired(cfg)

	return cfg
}

func validateRequired(cfg *AppConfig) {
	required := map[string]string{
		"HALOAI_API_URL":         cfg.HaloAIAPIURL,
		"WA_API_TOKEN":           cfg.WAAPIToken,
		"WA_WEBHOOK_SECRET":      cfg.WAWebhookSecret,
		"TELEGRAM_BOT_TOKEN":     cfg.TelegramBotToken,
		"TELEGRAM_AE_LEAD_ID":    cfg.TelegramAELeadID,
		"HANDOFF_WEBHOOK_SECRET": cfg.HandoffWebhookSecret,
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
