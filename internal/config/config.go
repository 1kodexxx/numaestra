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
	Env  string // dev | staging | production
	// Mode управляет тем, какие компоненты запускаются в процессе:
	//   all    — HTTP-сервер + Asynq-воркер (по умолчанию, обратно совместимо)
	//   api    — только HTTP-сервер (для горизонтального масштабирования API)
	//   worker — только Asynq-воркер (для горизонтального масштабирования воркеров)
	Mode string // all | api | worker
	HTTP HTTPConfig
	Postgres  PostgresConfig
	Redis     RedisConfig
	Robokassa RobokassaConfig
	Suno      SunoConfig
	S3        S3Config
	OpenAI    OpenAIConfig
	Pricing   PricingConfig
	Notify    NotifyConfig
	AdminToken string // ADMIN_TOKEN — Bearer-токен для /api/v1/admin/* маршрутов
	// SessionEncryptionKey — hex-строка из 64 символов (32 байта, AES-256).
	// Обязателен во всех окружениях, кроме dev (там используется небезопасный дев-ключ).
	SessionEncryptionKey string
}

type HTTPConfig struct {
	Port            string
	ShutdownTimeout time.Duration
	// MaxBodyBytes ограничивает размер тела входящего запроса для защиты от
	// исчерпания памяти крупными телами. 0 означает «без ограничения».
	MaxBodyBytes int64
	// CORSAllowedOrigins — список доменов, которым разрешён доступ к API.
	// Пустой список трактуется как "*" (любой источник) — удобно для dev.
	CORSAllowedOrigins []string
	// MetricsAllowedIPs — CIDR-блоки, с которых разрешён доступ к /metrics.
	// По умолчанию — только loopback. Пустой список открывает эндпоинт всем.
	MetricsAllowedIPs []string
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
	// AllowedIPs — список IP/CIDR, с которых принимаются вебхуки ResultURL.
	// Пустой список отключает фильтрацию по IP (подпись проверяется всегда).
	// Актуальные подсети Robokassa см. в их документации/поддержке.
	AllowedIPs []string
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

// NotifyConfig задаёт параметры отправки уведомлений клиентам.
// При пустом SMTPHost используется заглушка-логгер.
type NotifyConfig struct {
	SMTPHost     string // SMTP_HOST, например smtp.mailgun.org
	SMTPPort     int    // SMTP_PORT, дефолт 587 (STARTTLS)
	SMTPUser     string // SMTP_USER
	SMTPPassword string // SMTP_PASSWORD
	FromAddress  string // SMTP_FROM_ADDRESS, например noreply@numaestra.ru
	FromName     string // SMTP_FROM_NAME, например Numaestra
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
		Env:  getEnv("APP_ENV", "dev"),
		Mode: getEnv("APP_MODE", "all"),
		HTTP: HTTPConfig{
			Port:               getEnv("HTTP_PORT", "8080"),
			ShutdownTimeout:    getDurationEnv("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
			MaxBodyBytes:       getInt64Env("HTTP_MAX_BODY_BYTES", 1<<20), // 1 МБ по умолчанию
			CORSAllowedOrigins: getCSVEnv("CORS_ALLOWED_ORIGINS"),
			MetricsAllowedIPs:  getCSVEnvDefault("METRICS_ALLOWED_IPS", "127.0.0.1/8,::1/128"),
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
			// Дефолт false: в проде безопаснее «боевой» режим. Тестовый режим
			// нужно включать осознанно через ROBOKASSA_IS_TEST=true в dev-окружении,
			// иначе платежи уходят в тест и Robokassa их не зачисляет.
			IsTest:     getBoolEnv("ROBOKASSA_IS_TEST", false),
			AllowedIPs: getCSVEnv("ROBOKASSA_ALLOWED_IPS"),
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
		Notify: NotifyConfig{
			SMTPHost:     getEnv("SMTP_HOST", ""),
			SMTPPort:     int(getInt64Env("SMTP_PORT", 587)),
			SMTPUser:     getEnv("SMTP_USER", ""),
			SMTPPassword: getEnv("SMTP_PASSWORD", ""),
			FromAddress:  getEnv("SMTP_FROM_ADDRESS", ""),
			FromName:     getEnv("SMTP_FROM_NAME", "Numaestra"),
		},
		AdminToken:           getEnv("ADMIN_TOKEN", ""),
		SessionEncryptionKey: getEnv("SESSION_ENCRYPTION_KEY", ""),
	}

	if cfg.Postgres.DSN == "" {
		return nil, fmt.Errorf("POSTGRES_DSN является обязательным параметром")
	}

	if cfg.Env != "dev" && cfg.AdminToken == "" {
		return nil, fmt.Errorf("ADMIN_TOKEN обязателен в окружении %q (сгенерировать: openssl rand -hex 32)", cfg.Env)
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

// getCSVEnv парсит переменную окружения как список значений через запятую.
// Пустые элементы и окружающие пробелы отбрасываются. Возвращает nil, если
// переменная не задана или пуста.
func getCSVEnvDefault(key, fallback string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		v = fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getCSVEnv(key string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func getBoolEnv(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	vLower := strings.ToLower(v)
	return vLower == "1" || vLower == "true" || vLower == "yes"
}
