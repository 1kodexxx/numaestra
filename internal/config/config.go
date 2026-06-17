package config

import (
	"fmt"
	"os"
	"time"
)

// Config - конфигурация приложения, загружаемая из переменных окружения.
// На проде значения приходят из секрет-менеджера (Vault, Kubernetes Secrets и т.д.).
type Config struct {
	Env      string // dev | staging | production
	HTTP     HTTPConfig
	Postgres PostgresConfig
	Redis    RedisConfig
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

// Load читает конфигурацию из переменных окружения с безопасными дефолтами для dev.
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
	}

	if cfg.Postgres.DSN == "" {
		return nil, fmt.Errorf("POSTGRES_DSN не может быть пустым")
	}

	return cfg, nil
}

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
