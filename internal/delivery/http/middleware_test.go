package apphttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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

func TestCORS_SpecificOrigin_SetsCredentials(t *testing.T) {
	h := CORS(DefaultCORSOptions([]string{"https://app.numaestra.com"}))(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.numaestra.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("ожидали Allow-Credentials=true для разрешённого origin, получили %q", got)
	}
}

func TestCORS_AllowAll_NoCredentials(t *testing.T) {
	h := CORS(DefaultCORSOptions(nil))(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Allow-Credentials не должен выставляться вместе с '*' (запрещено спецификацией), получили %q", got)
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

func TestMaxBodyBytes_RejectsOversizedBody(t *testing.T) {
	// Хендлер пытается прочитать тело целиком; при превышении лимита чтение
	// должно вернуть ошибку, и хендлер ответит 413.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	h := MaxBodyBytes(10)(handler)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", 100)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("ожидали 413 при превышении лимита тела, получили %d", rec.Code)
	}
}

func TestMaxBodyBytes_AllowsSmallBody(t *testing.T) {
	h := MaxBodyBytes(1024)(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("small"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("малое тело должно проходить, получили %d", rec.Code)
	}
}

func TestIPAllowlist_AllowsConfiguredAndBlocksOthers(t *testing.T) {
	nets, err := ParseCIDRs([]string{"185.59.216.0/24", "1.2.3.4"})
	if err != nil {
		t.Fatalf("ParseCIDRs упал: %v", err)
	}
	h := IPAllowlist(nets)(okHandler())

	do := func(ip string) int {
		req := httptest.NewRequest(http.MethodPost, "/webhook/robokassa", nil)
		req.RemoteAddr = ip + ":40000"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := do("185.59.216.10"); code != http.StatusOK {
		t.Errorf("IP из разрешённой подсети должен проходить, получили %d", code)
	}
	if code := do("1.2.3.4"); code != http.StatusOK {
		t.Errorf("разрешённый одиночный IP должен проходить, получили %d", code)
	}
	if code := do("8.8.8.8"); code != http.StatusForbidden {
		t.Errorf("посторонний IP должен получать 403, получили %d", code)
	}
}

func TestIPAllowlist_EmptyDisablesFilter(t *testing.T) {
	h := IPAllowlist(nil)(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/webhook/robokassa", nil)
	req.RemoteAddr = "8.8.8.8:40000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("пустой список должен пропускать любой IP, получили %d", rec.Code)
	}
}

func TestParseCIDRs_RejectsInvalid(t *testing.T) {
	if _, err := ParseCIDRs([]string{"not-an-ip"}); err == nil {
		t.Error("ожидали ошибку для некорректной записи")
	}
}

func TestClientIP_WithoutPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1" // без порта — SplitHostPort вернёт ошибку
	ip := clientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("ожидали %q, получили %q", "192.168.1.1", ip)
	}
}

func TestItoa_Zero(t *testing.T) {
	if got := itoa(0); got != "0" {
		t.Errorf("itoa(0) = %q, ожидали \"0\"", got)
	}
}

func TestItoa_Negative(t *testing.T) {
	if got := itoa(-42); got != "-42" {
		t.Errorf("itoa(-42) = %q, ожидали \"-42\"", got)
	}
}

func TestParseCIDRs_InvalidCIDR(t *testing.T) {
	if _, err := ParseCIDRs([]string{"192.168.1.1/999"}); err == nil {
		t.Error("ожидали ошибку для некорректного CIDR")
	}
}

func TestCORS_NoOriginHeader_NoHeadersSet(t *testing.T) {
	h := CORS(DefaultCORSOptions([]string{"https://allowed.com"}))(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/", nil) // нет заголовка Origin
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("без Origin заголовок CORS не должен выставляться")
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

// --- DistributedRateLimiter ---

func redisClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mini := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return mini, rdb
}

func doRequest(h http.Handler, ip string) int {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestDistributedRateLimiter_AllowsWithinLimit(t *testing.T) {
	_, rdb := redisClient(t)
	h := DistributedRateLimiter(rdb, 3, time.Minute)(okHandler())

	for i := 1; i <= 3; i++ {
		if code := doRequest(h, "10.0.0.1"); code != http.StatusOK {
			t.Errorf("запрос %d: ожидали 200, получили %d", i, code)
		}
	}
}

func TestDistributedRateLimiter_BlocksOverLimit(t *testing.T) {
	_, rdb := redisClient(t)
	h := DistributedRateLimiter(rdb, 2, time.Minute)(okHandler())

	doRequest(h, "10.0.0.2") //nolint:errcheck
	doRequest(h, "10.0.0.2") //nolint:errcheck

	if code := doRequest(h, "10.0.0.2"); code != http.StatusTooManyRequests {
		t.Errorf("третий запрос при лимите 2: ожидали 429, получили %d", code)
	}
}

func TestDistributedRateLimiter_SeparateBucketsPerIP(t *testing.T) {
	_, rdb := redisClient(t)
	h := DistributedRateLimiter(rdb, 1, time.Minute)(okHandler())

	if code := doRequest(h, "10.0.0.3"); code != http.StatusOK {
		t.Fatalf("первый IP: ожидали 200, получили %d", code)
	}
	// Второй IP имеет свой бакет — должен пройти.
	if code := doRequest(h, "10.0.0.4"); code != http.StatusOK {
		t.Fatalf("второй IP должен иметь независимый бакет, получили %d", code)
	}
}

func TestDistributedRateLimiter_FailOpen_WhenRedisDown(t *testing.T) {
	mini, rdb := redisClient(t)
	mini.Close() // Redis недоступен до первого запроса

	h := DistributedRateLimiter(rdb, 1, time.Minute)(okHandler())

	// Должен пройти (fail open).
	if code := doRequest(h, "10.0.0.5"); code != http.StatusOK {
		t.Errorf("при недоступном Redis ожидали fail-open (200), получили %d", code)
	}
}
