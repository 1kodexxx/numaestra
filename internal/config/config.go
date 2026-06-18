package config

import (
	"fmt"
	"os"
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
	OpenAI    OpenAIConfig
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

type OpenAIConfig struct {
	APIKey string
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
		OpenAI: OpenAIConfig{
			APIKey: getEnv("OPENAI_API_KEY", ""),
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

func getBoolEnv(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	vLower := strings.ToLower(v)
	return vLower == "1" || vLower == "true" || vLower == "yes"
}
