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
	if !cfg.Robokassa.IsTest {
		t.Error("по умолчанию ROBOKASSA_IS_TEST должен быть true")
	}
}

func TestLoad_OverridesFromEnv(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "30s")
	t.Setenv("ROBOKASSA_IS_TEST", "false")
	t.Setenv("SUNO_API_KEY", "secret-key")

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
	if cfg.Suno.APIKey != "secret-key" {
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
