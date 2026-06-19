package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config - главная структура конфигурации, собирающая настройки всех подсистем.
type Config struct {
	Env       string // dev | staging | production
	HTTP      HTTPConfig
	Postgres  PostgresConfig
	Redis     RedisConfig
	Robokassa RobokassaConfig
	Suno      SunoConfig
	S3        S3Config
	OpenAI    OpenAIConfig
	Pricing   PricingConfig
}

type HTTPConfig struct {
	Port            string
	ShutdownTimeout time.Duration
}

type PostgresConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr     string
	Password string
}

type RobokassaConfig struct {
	MerchantLogin string
	Password1     string // Для генерации платежной ссылки
	Password2     string // Для проверки подписи вебхука
	IsTest        bool   // Флаг тестового режима
}

type SunoConfig struct {
	APIURL string
	APIKey string
}

type S3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

type OpenAIConfig struct {
	BaseURL string // По умолчанию OpenRouter; можно переключить на прямой OpenAI
	APIKey  string
}

// PricingConfig задаёт серверный прайс. Цены НЕ принимаются от клиента —
// клиент выбирает только тариф (plan), а сумму определяет сервер.
type PricingConfig struct {
	Plans       map[string]int64 // тариф -> цена в копейках
	DefaultPlan string           // тариф по умолчанию, если клиент не указал
}

// Load считывает переменные окружения и собирает их в структуру Config.
// Если обязательные переменные отсутствуют, возвращает ошибку.
func Load() (*Config, error) {
	cfg := &Config{
		Env: getEnv("APP_ENV", "dev"),
		HTTP: HTTPConfig{
			Port:            getEnv("HTTP_PORT", "8080"),
			ShutdownTimeout: getDurationEnv("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		Postgres: PostgresConfig{
			DSN: getEnv("POSTGRES_DSN", "postgres://numaestra:numaestra@localhost:5432/numaestra?sslmode=disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		Robokassa: RobokassaConfig{
			MerchantLogin: getEnv("ROBOKASSA_MERCHANT_LOGIN", "numaestra_test"),
			Password1:     getEnv("ROBOKASSA_PASS1", "test_pass1"),
			Password2:     getEnv("ROBOKASSA_PASS2", "test_pass2"),
			IsTest:        getBoolEnv("ROBOKASSA_IS_TEST", true),
		},
		Suno: SunoConfig{
			APIURL: getEnv("SUNO_API_URL", "https://api.custom-suno.local"),
			APIKey: getEnv("SUNO_API_KEY", ""),
		},
		S3: S3Config{
			Endpoint:  getEnv("S3_ENDPOINT", "https://s3.amazonaws.com"),
			Region:    getEnv("S3_REGION", "us-east-1"),
			Bucket:    getEnv("S3_BUCKET", "numaestra-tracks"),
			AccessKey: getEnv("S3_ACCESS_KEY", ""),
			SecretKey: getEnv("S3_SECRET_KEY", ""),
		},
		OpenAI: OpenAIConfig{
			BaseURL: getEnv("OPENAI_BASE_URL", "https://openrouter.ai/api/v1"),
			APIKey:  getEnv("OPENAI_API_KEY", ""),
		},
		Pricing: PricingConfig{
			// Тарифы и их цены задаются сервером. Стандартный тариф — 4 версии песни.
			Plans: map[string]int64{
				"standard": getInt64Env("PRICE_STANDARD_KOPECKS", 150000),
				"premium":  getInt64Env("PRICE_PREMIUM_KOPECKS", 290000),
			},
			DefaultPlan: getEnv("PRICE_DEFAULT_PLAN", "standard"),
		},
	}

	if cfg.Postgres.DSN == "" {
		return nil, fmt.Errorf("POSTGRES_DSN является обязательным параметром")
	}

	return cfg, nil
}

// --- Вспомогательные функции для парсинга ---

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func getInt64Env(key string, fallback int64) int64 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func getBoolEnv(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	vLower := strings.ToLower(v)
	return vLower == "1" || vLower == "true" || vLower == "yes"
}
