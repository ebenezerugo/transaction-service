package config

import (
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Kafka    KafkaConfig
	NIBSS    NIBSSConfig
	Fineract FineractConfig
	Retry    RetryConfig
}

type ServerConfig struct {
	Port string
	Mode string
}

type DatabaseConfig struct {
	DSN            string
	MaxConnections int
	MinConnections int
}

type KafkaConfig struct {
	Brokers       string
	GroupID       string
	NumPartitions int
}

type NIBSSConfig struct {
	BaseURL   string
	APIKey    string
	SecretKey string
	Timeout   int
}

type FineractConfig struct {
	BaseURL  string
	Username string
	Password string
	TenantID string
	Timeout  int
}

type RetryConfig struct {
	Enabled          bool
	MaxAttempts      int
	BackoffStrategy  string
	InitialBackoffMs int
	MaxBackoffMs     int
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Mode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			DSN:            getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/transaction_service?sslmode=disable"),
			MaxConnections: getEnvInt("DB_MAX_CONNECTIONS", 25),
			MinConnections: getEnvInt("DB_MIN_CONNECTIONS", 5),
		},
		Kafka: KafkaConfig{
			Brokers:       getEnv("KAFKA_BROKERS", "localhost:9092"),
			GroupID:       getEnv("KAFKA_GROUP_ID", "transaction-service"),
			NumPartitions: getEnvInt("KAFKA_NUM_PARTITIONS", 12),
		},
		NIBSS: NIBSSConfig{
			BaseURL:   getEnv("NIBSS_BASE_URL", ""),
			APIKey:    getEnv("NIBSS_API_KEY", ""),
			SecretKey: getEnv("NIBSS_SECRET_KEY", ""),
			Timeout:   getEnvInt("NIBSS_TIMEOUT_SECONDS", 30),
		},
		Fineract: FineractConfig{
			BaseURL:  getEnv("FINERACT_BASE_URL", ""),
			Username: getEnv("FINERACT_USERNAME", ""),
			Password: getEnv("FINERACT_PASSWORD", ""),
			TenantID: getEnv("FINERACT_TENANT_ID", "default"),
			Timeout:  getEnvInt("FINERACT_TIMEOUT_SECONDS", 30),
		},
		Retry: RetryConfig{
			Enabled:          getEnvBool("RETRY_ENABLED", true),
			MaxAttempts:      getEnvInt("MAX_RETRY_ATTEMPTS", 3),
			BackoffStrategy:  getEnv("BACKOFF_STRATEGY", "exponential"),
			InitialBackoffMs: getEnvInt("BACKOFF_INITIAL_MS", 1000),
			MaxBackoffMs:     getEnvInt("BACKOFF_MAX_MS", 30000),
		},
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}
