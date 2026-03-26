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

	// Event Dispatcher Settings
	DispatcherEnabled      bool
	DispatcherPollInterval time.Duration
	DispatcherHTTPTimeout  time.Duration
	DispatcherBatchSize    int

	// Webhook Settings
	WebhookDebugLogging bool

	// OpenTelemetry Tracing
	TracerExporter            string
	TracerZipkinCreateSpanURL string
	TracerServiceName         string
	TracerServiceVersion      string

	// GCP Configuration (auto-detected if empty)
	GCPProject string

	// Error Handling
	EnableStackTrace bool
}

func LoadConfig() *AppConfig {
	// Load from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system env")
	}

	return &AppConfig{
		// Application
		Env:         getEnv("ENV", "development"),
		Port:        getEnv("APP_PORT", "3000"),
		RoutePrefix: getEnv("APP_ROUTE_PREFIX", "/v1/cs-agent-bot"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		// PostgreSQL Connection
		DBEnabled:  getEnv("DB_ENABLED", "false"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "cs_agent_bot"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		// PostgreSQL Pool Settings
		DBMaxOpenConns:        getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:        getEnvInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime:     getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		DBConnMaxIdleTime:     getEnvDuration("DB_CONN_MAX_IDLE_TIME", 1*time.Minute),
		DBQueryTimeout:        getEnvDuration("DB_QUERY_TIMEOUT", 30*time.Second),
		DBStatsLoggingEnabled: getEnvBool("DB_STATS_LOGGING_ENABLED", false),

		// Redis Connection
		RedisEnabled:  getEnv("REDIS_ENABLED", "false"),
		RedisDB:       getEnv("REDIS_DB", "0"),
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisUsername: getEnv("REDIS_USERNAME", ""),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		// Event Dispatcher Settings
		DispatcherEnabled:      getEnvBool("DISPATCHER_ENABLED", true),
		DispatcherPollInterval: getEnvDuration("DISPATCHER_POLL_INTERVAL", 5*time.Second),
		DispatcherHTTPTimeout:  getEnvDuration("DISPATCHER_HTTP_TIMEOUT", 30*time.Second),
		DispatcherBatchSize:    getEnvInt("DISPATCHER_BATCH_SIZE", 100),

		// Webhook Settings
		WebhookDebugLogging: getEnvBool("WEBHOOK_DEBUG_LOGGING", false),

		// OpenTelemetry Tracing
		TracerExporter:            getEnv("TRACER_EXPORTER", "zipkin"),
		TracerZipkinCreateSpanURL: getEnv("TRACER_ZIPKIN_CREATE_SPAN_URL", "http://localhost:9411/api/v2/spans"),
		TracerServiceName:         getEnv("TRACER_SERVICE_NAME", "cs-agent-bot"),
		TracerServiceVersion:      getEnv("TRACER_SERVICE_VERSION", "v1.0.0"),

		// GCP Configuration
		GCPProject: getEnv("GCP_PROJECT", ""),

		// Error Handling
		EnableStackTrace: getEnvBool("ENABLE_STACK_TRACE", false),
	}
}

// getEnv retrieves a string environment variable or returns the default value
func getEnv(key, defaultVal string) string {
	if val, exists := os.LookupEnv(key); exists {
		return val
	}
	return defaultVal
}

// getEnvInt retrieves an integer environment variable or returns the default value
func getEnvInt(key string, defaultVal int) int {
	if val, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

// getEnvBool retrieves a boolean environment variable or returns the default value
func getEnvBool(key string, defaultVal bool) bool {
	if val, exists := os.LookupEnv(key); exists {
		if boolVal, err := strconv.ParseBool(val); err == nil {
			return boolVal
		}
	}
	return defaultVal
}

// getEnvDuration retrieves a duration environment variable or returns the default value
// Supports formats: "30s", "5m", "1h", etc.
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(val); err == nil {
			return duration
		}
	}
	return defaultVal
}
