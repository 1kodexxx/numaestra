package apphttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestCORS_AllowAll_SetsHeader(t *testing.T) {
	h := CORS(DefaultCORSOptions(nil))(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ожидали '*' в Allow-Origin, получили %q", got)
	}
}

func TestCORS_Preflight_ShortCircuits(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	})
	h := CORS(DefaultCORSOptions([]string{"https://app.numaestra.com"}))(next)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://app.numaestra.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("preflight должен вернуть 204, получили %d", rec.Code)
	}
	if called {
		t.Error("preflight не должен доходить до основного хендлера")
	}
}

func TestCORS_SpecificOrigin_Reflected(t *testing.T) {
	h := CORS(DefaultCORSOptions([]string{"https://app.numaestra.com"}))(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.numaestra.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.numaestra.com" {
		t.Errorf("ожидали отражение разрешённого Origin, получили %q", got)
	}

	// Чужой Origin не должен отражаться.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "https://evil.com")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	if got := rec2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("неразрешённый Origin не должен отражаться, получили %q", got)
	}
}

func TestRateLimiter_BlocksAfterBurst(t *testing.T) {
	// rps=1, burst=2 — первые 2 запроса проходят, третий блокируется.
	h := RateLimiter(1, 2)(okHandler())

	do := func() int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do(); code != http.StatusOK {
		t.Fatalf("1-й запрос должен пройти, получили %d", code)
	}
	if code := do(); code != http.StatusOK {
		t.Fatalf("2-й запрос должен пройти (burst=2), получили %d", code)
	}
	if code := do(); code != http.StatusTooManyRequests {
		t.Fatalf("3-й запрос должен быть отклонён 429, получили %d", code)
	}
}

func TestRateLimiter_SeparatePerIP(t *testing.T) {
	h := RateLimiter(1, 1)(okHandler())

	do := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":5000"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do("1.1.1.1"); code != http.StatusOK {
		t.Fatalf("первый IP должен пройти, получили %d", code)
	}
	// Другой IP не должен затрагиваться лимитом первого.
	if code := do("2.2.2.2"); code != http.StatusOK {
		t.Fatalf("второй IP должен иметь свой бакет, получили %d", code)
	}
}
