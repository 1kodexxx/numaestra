package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Очищаем все известные переменные, чтобы проверить значения по умолчанию.
	for _, k := range []string{
		"APP_ENV", "HTTP_PORT", "HTTP_SHUTDOWN_TIMEOUT", "POSTGRES_DSN",
		"REDIS_ADDR", "ROBOKASSA_IS_TEST", "SUNO_API_URL",
	} {
		t.Setenv(k, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load упал: %v", err)
	}
	if cfg.Env != "dev" {
		t.Errorf("ожидали env=dev, получили %q", cfg.Env)
	}
	if cfg.HTTP.Port != "8080" {
		t.Errorf("ожидали порт 8080, получили %q", cfg.HTTP.Port)
	}
	if cfg.HTTP.ShutdownTimeout != 15*time.Second {
		t.Errorf("ожидали shutdown 15s, получили %v", cfg.HTTP.ShutdownTimeout)
	}
	if cfg.Robokassa.IsTest {
		t.Error("по умолчанию ROBOKASSA_IS_TEST должен быть false (безопасный продакшен-дефолт)")
	}
	if cfg.Pricing.PriceKopecks != 200000 {
		t.Errorf("дефолтная цена должна быть 200000 (2000 ₽), получили %d", cfg.Pricing.PriceKopecks)
	}
}

func TestLoad_PricingOverride(t *testing.T) {
	t.Setenv("PRICE_KOPECKS", "350000")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load упал: %v", err)
	}
	if cfg.Pricing.PriceKopecks != 350000 {
		t.Errorf("ожидали переопределённую цену 350000, получили %d", cfg.Pricing.PriceKopecks)
	}
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("ROBOKASSA_IS_TEST", "false")
	t.Setenv("SUNO_API_KEY", "secret-suno-key")
	t.Setenv("OPENAI_API_KEY", "secret-openai-key")
	t.Setenv("S3_ACCESS_KEY", "s3-access")
	t.Setenv("S3_SECRET_KEY", "s3-secret")
	t.Setenv("ADMIN_TOKEN", "test-admin-token")
	t.Setenv("ADMIN_LOGIN", "owner")
	t.Setenv("ADMIN_PASSWORD", "test-admin-password")
	t.Setenv("ADMIN_SESSION_SECRET", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd")
	t.Setenv("ROBOKASSA_MERCHANT_LOGIN", "real_shop")
	t.Setenv("ROBOKASSA_PASS1", "real-pass-1")
	t.Setenv("ROBOKASSA_PASS2", "real-pass-2")
	t.Setenv("ROBOKASSA_ALLOWED_IPS", "185.59.216.0/24")
	t.Setenv("REDIS_PASSWORD", "redis-secret")
	t.Setenv("SESSION_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load упал: %v", err)
	}
	if cfg.Env != "production" {
		t.Errorf("ожидали production, получили %q", cfg.Env)
	}
	if cfg.HTTP.Port != "9090" {
		t.Errorf("ожидали 9090, получили %q", cfg.HTTP.Port)
	}
	if cfg.HTTP.ShutdownTimeout != 30*time.Second {
		t.Errorf("ожидали 30s, получили %v", cfg.HTTP.ShutdownTimeout)
	}
	if cfg.Robokassa.IsTest {
		t.Error("ROBOKASSA_IS_TEST=false должен дать false")
	}
	if cfg.Suno.APIKey != "secret-suno-key" {
		t.Errorf("ожидали suno key, получили %q", cfg.Suno.APIKey)
	}
}

func TestLoad_InvalidDurationFallsBack(t *testing.T) {
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "не-длительность")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load упал: %v", err)
	}
	if cfg.HTTP.ShutdownTimeout != 15*time.Second {
		t.Errorf("при некорректной длительности должен использоваться дефолт 15s, получили %v", cfg.HTTP.ShutdownTimeout)
	}
}

func TestLoad_CSVEnvOverride(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app1.com, https://app2.com, ")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load упал: %v", err)
	}
	if len(cfg.HTTP.CORSAllowedOrigins) != 2 {
		t.Errorf("ожидали 2 origin (пустые отбрасываются), получили %d: %v",
			len(cfg.HTTP.CORSAllowedOrigins), cfg.HTTP.CORSAllowedOrigins)
	}
}

func TestLoad_AdminTokenRequired_NonDev(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("ADMIN_TOKEN", "")
	_, err := Load()
	if err == nil {
		t.Fatal("ожидали ошибку: ADMIN_TOKEN обязателен в staging")
	}
}

func TestLoad_SunoKeyRequired_NonDev(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("ADMIN_TOKEN", "tok")
	t.Setenv("ADMIN_LOGIN", "owner")
	t.Setenv("ADMIN_PASSWORD", "pass")
	t.Setenv("ADMIN_SESSION_SECRET", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd")
	t.Setenv("SUNO_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("S3_ACCESS_KEY", "s3-access")
	t.Setenv("S3_SECRET_KEY", "s3-secret")
	_, err := Load()
	if err == nil {
		t.Fatal("ожидали ошибку: SUNO_API_KEY обязателен в staging")
	}
}

func TestLoad_OpenAIOptional_NonDev(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("ADMIN_TOKEN", "tok")
	t.Setenv("ADMIN_LOGIN", "owner")
	t.Setenv("ADMIN_PASSWORD", "pass")
	t.Setenv("ADMIN_SESSION_SECRET", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd")
	t.Setenv("SUNO_API_KEY", "suno-key")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("REDIS_PASSWORD", "redis-secret")
	t.Setenv("SESSION_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd")
	t.Setenv("S3_ACCESS_KEY", "s3-access")
	t.Setenv("S3_SECRET_KEY", "s3-secret")
	t.Setenv("ROBOKASSA_MERCHANT_LOGIN", "real-shop")
	t.Setenv("ROBOKASSA_PASS1", "real-pass1")
	t.Setenv("ROBOKASSA_PASS2", "real-pass2")
	t.Setenv("ROBOKASSA_ALLOWED_IPS", "127.0.0.1")
	if _, err := Load(); err != nil {
		t.Fatalf("OPENAI_API_KEY не обязателен при отключённом LLM: %v", err)
	}
}

func TestLoad_S3KeysRequired_NonDev(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("ADMIN_TOKEN", "tok")
	t.Setenv("ADMIN_LOGIN", "owner")
	t.Setenv("ADMIN_PASSWORD", "pass")
	t.Setenv("ADMIN_SESSION_SECRET", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd")
	t.Setenv("SUNO_API_KEY", "suno-key")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("S3_ACCESS_KEY", "")
	t.Setenv("S3_SECRET_KEY", "")
	_, err := Load()
	if err == nil {
		t.Fatal("ожидали ошибку: S3-ключи обязательны в staging")
	}
}

func TestLoad_AdminLoginPasswordRequired_NonDev(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("ADMIN_TOKEN", "tok")
	t.Setenv("ADMIN_LOGIN", "")
	t.Setenv("ADMIN_PASSWORD", "")
	_, err := Load()
	if err == nil {
		t.Fatal("ожидали ошибку: ADMIN_LOGIN/ADMIN_PASSWORD обязательны в staging")
	}
}

func TestLoad_AdminSessionSecretRequired_NonDev(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("ADMIN_TOKEN", "tok")
	t.Setenv("ADMIN_LOGIN", "owner")
	t.Setenv("ADMIN_PASSWORD", "pass")
	t.Setenv("ADMIN_SESSION_SECRET", "")
	_, err := Load()
	if err == nil {
		t.Fatal("ожидали ошибку: ADMIN_SESSION_SECRET обязателен в staging")
	}
}

// setValidNonDevEnv выставляет полный набор обязательных переменных для
// успешной загрузки конфига в не-dev окружении.
func setValidNonDevEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "staging")
	t.Setenv("ADMIN_TOKEN", "tok")
	t.Setenv("ADMIN_LOGIN", "owner")
	t.Setenv("ADMIN_PASSWORD", "pass")
	t.Setenv("ADMIN_SESSION_SECRET", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd")
	t.Setenv("SUNO_API_KEY", "suno-key")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("S3_ACCESS_KEY", "s3-access")
	t.Setenv("S3_SECRET_KEY", "s3-secret")
	t.Setenv("ROBOKASSA_MERCHANT_LOGIN", "real_shop")
	t.Setenv("ROBOKASSA_PASS1", "real-pass-1")
	t.Setenv("ROBOKASSA_PASS2", "real-pass-2")
	t.Setenv("ROBOKASSA_ALLOWED_IPS", "185.59.216.0/24")
	t.Setenv("ROBOKASSA_IS_TEST", "false")
	t.Setenv("REDIS_PASSWORD", "redis-secret")
	t.Setenv("SESSION_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd")
}

func TestLoad_InvalidAppMode(t *testing.T) {
	t.Setenv("APP_MODE", "bogus")
	_, err := Load()
	if err == nil {
		t.Fatal("ожидали ошибку при неверном APP_MODE")
	}
}

func TestLoad_NonDev_Success(t *testing.T) {
	setValidNonDevEnv(t)
	if _, err := Load(); err != nil {
		t.Fatalf("ожидали успешную загрузку валидного prod-конфига, получили: %v", err)
	}
}

func TestLoad_RobokassaTestPasswordsRejected_NonDev(t *testing.T) {
	setValidNonDevEnv(t)
	t.Setenv("ROBOKASSA_PASS2", "test_pass2") // публичная тестовая заглушка
	if _, err := Load(); err == nil {
		t.Fatal("ожидали ошибку: тестовый ROBOKASSA_PASS2 недопустим в prod")
	}
}

func TestLoad_RobokassaLoginDefaultRejected_NonDev(t *testing.T) {
	setValidNonDevEnv(t)
	t.Setenv("ROBOKASSA_MERCHANT_LOGIN", "numaestra_test")
	if _, err := Load(); err == nil {
		t.Fatal("ожидали ошибку: тестовый ROBOKASSA_MERCHANT_LOGIN недопустим в prod")
	}
}

func TestLoad_RobokassaAllowedIPsRequired_NonDev(t *testing.T) {
	setValidNonDevEnv(t)
	t.Setenv("ROBOKASSA_ALLOWED_IPS", "")
	if _, err := Load(); err == nil {
		t.Fatal("ожидали ошибку: ROBOKASSA_ALLOWED_IPS обязателен в prod")
	}
}

func TestLoad_RobokassaIsTestRejected_NonDev(t *testing.T) {
	setValidNonDevEnv(t)
	t.Setenv("ROBOKASSA_IS_TEST", "true")
	if _, err := Load(); err == nil {
		t.Fatal("ожидали ошибку: ROBOKASSA_IS_TEST=true недопустим в prod")
	}
}

func TestLoad_PublicAppURLRequiredWhenSMTP_NonDev(t *testing.T) {
	setValidNonDevEnv(t)
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("PUBLIC_APP_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("ожидали ошибку: PUBLIC_APP_URL обязателен при SMTP_HOST в non-dev")
	}
}

func TestGetBoolEnv_Variants(t *testing.T) {
	cases := map[string]bool{
		"1": true, "true": true, "TRUE": true, "yes": true,
		"0": false, "false": false, "no": false, "garbage": false,
	}
	for v, want := range cases {
		t.Setenv("ROBOKASSA_IS_TEST", v)
		cfg, _ := Load()
		if cfg.Robokassa.IsTest != want {
			t.Errorf("getBoolEnv(%q) = %v, хотели %v", v, cfg.Robokassa.IsTest, want)
		}
	}
}
