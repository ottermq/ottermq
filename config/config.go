package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Core
	ShowLogo   bool
	BrokerPort string
	BrokerHost string
	DataDir    string

	// AMQP Settings
	HeartbeatIntervalMax uint16
	ChannelMax           uint16
	FrameMax             uint32
	Version              string
	Ssl                  bool
	QueueBufferSize      int
	MaxPriority          uint8

	// Extensions
	EnableDLX bool // Dead-Letter Exchange
	EnableTTL bool // Time-To-Live
	EnableQLL bool // Queue Length Limit

	// Plugins
	EnableWebAPI  bool
	EnableUI      bool
	EnableSwagger bool

	WebAPIPath  string
	SwaggerPath string

	// Web Admin
	WebPort   string
	Username  string
	Password  string
	JwtSecret string

	// Logging
	LogLevel string

	// Metrics

	EnableMetrics   bool          // Enable or disable metrics collection
	WindowSize      time.Duration // Time window for rate calculations
	MaxSamples      int           // Maximum number of samples to retain
	SamplesInterval uint8         // Interval between samples (in seconds) default 5s
}

// LoadConfig loads configuration from .env file, environment variables, or defaults
// Priority: environment variables > .env file > default values
func LoadConfig(version string) *Config {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	return &Config{
		ShowLogo:   getEnvAsBool("OTTERMQ_SHOW_LOGO", false),
		BrokerPort: getEnv("OTTERMQ_BROKER_PORT", "5672"),
		BrokerHost: getEnv("OTTERMQ_BROKER_HOST", ""),
		DataDir:    getEnv("OTTERMQ_DATA_DIR", "data"),

		HeartbeatIntervalMax: getEnvAsUint16("OTTERMQ_HEARTBEAT_INTERVAL", 60),
		ChannelMax:           getEnvAsUint16("OTTERMQ_CHANNEL_MAX", 2048),
		FrameMax:             getEnvAsUint32("OTTERMQ_FRAME_MAX", 131072),
		Ssl:                  getEnvAsBool("OTTERMQ_SSL", false),
		QueueBufferSize:      getEnvAsInt("OTTERMQ_QUEUE_BUFFER_SIZE", 100000),
		MaxPriority:          getEnvAsUint8("OTTERMQ_MAX_PRIORITY", 10), // 0-255 (default 10)

		EnableDLX: getEnvAsBool("OTTERMQ_ENABLE_DLX", true),
		EnableTTL: getEnvAsBool("OTTERMQ_ENABLE_TTL", true),
		EnableQLL: getEnvAsBool("OTTERMQ_ENABLE_QLL", true),

		EnableWebAPI:  getEnvAsBool("OTTERMQ_ENABLE_WEB_API", true),
		EnableUI:      getEnvAsBool("OTTERMQ_ENABLE_UI", true),
		EnableSwagger: getEnvAsBool("OTTERMQ_ENABLE_SWAGGER", false),
		WebAPIPath:    getEnv("OTTERMQ_WEB_API_PATH", "/api"),
		SwaggerPath:   getEnv("OTTERMQ_SWAGGER_PATH", "/docs"),

		WebPort:   getEnv("OTTERMQ_WEB_PORT", "3000"),
		Username:  getEnv("OTTERMQ_USERNAME", "guest"),
		Password:  getEnv("OTTERMQ_PASSWORD", "guest"),
		JwtSecret: getEnv("OTTERMQ_JWT_SECRET", "secret"),
		Version:   version,

		LogLevel: getEnv("LOG_LEVEL", "info"),

		EnableMetrics:   getEnvAsBool("OTTERMQ_ENABLE_METRICS", true),
		WindowSize:      getEnvAsDuration("OTTERMQ_METRICS_WINDOW_SIZE", 5*time.Minute),
		MaxSamples:      getEnvAsInt("OTTERMQ_METRICS_MAX_SAMPLES", 60),
		SamplesInterval: getEnvAsUint8("OTTERMQ_METRICS_SAMPLES_INTERVAL", 5), // seconds
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		fmt.Printf("Warning: Invalid value for %s: %s, using default: %d\n", key, valueStr, defaultValue)
		return defaultValue
	}
	return value
}

func getEnvAsUint16(key string, defaultValue uint16) uint16 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseUint(valueStr, 10, 16)
	if err != nil {
		fmt.Printf("Warning: Invalid value for %s: %s, using default: %d\n", key, valueStr, defaultValue)
		return defaultValue
	}
	return uint16(value)
}

func getEnvAsUint8(key string, defaultValue uint8) uint8 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseUint(valueStr, 10, 16)
	if err != nil {
		fmt.Printf("Warning: Invalid value for %s: %s, using default: %d\n", key, valueStr, defaultValue)
		return defaultValue
	}
	if value > uint64(^uint8(0)) {
		fmt.Printf("Warning: Value for %s exceeds max (%d), clamping to max value\n", key, ^uint8(0))
		return ^uint8(0)
	}
	return uint8(value)
}

func getEnvAsUint32(key string, defaultValue uint32) uint32 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseUint(valueStr, 10, 32)
	if err != nil {
		fmt.Printf("Warning: Invalid value for %s: %s, using default: %d\n", key, valueStr, defaultValue)
		return defaultValue
	}
	return uint32(value)
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		fmt.Printf("Warning: Invalid value for %s: %s, using default: %t\n", key, valueStr, defaultValue)
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		fmt.Printf("Warning: Invalid value for %s: %s, using default: %s\n", key, valueStr, defaultValue)
		return defaultValue
	}
	return value
}
