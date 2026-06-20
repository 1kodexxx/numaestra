package apphttp

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ParseCIDRs преобразует список строк (IP или CIDR) в подсети net.IPNet.
// Одиночный IP трактуется как /32 (IPv4) либо /128 (IPv6). Возвращает ошибку,
// если хотя бы одна запись некорректна, чтобы неверная конфигурация не привела
// к молчаливому пропуску всех адресов.
func ParseCIDRs(entries []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(entries))
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if strings.Contains(e, "/") {
			_, ipNet, err := net.ParseCIDR(e)
			if err != nil {
				return nil, fmt.Errorf("некорректный CIDR %q: %w", e, err)
			}
			nets = append(nets, ipNet)
			continue
		}
		ip := net.ParseIP(e)
		if ip == nil {
			return nil, fmt.Errorf("некорректный IP %q", e)
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return nets, nil
}

// IPAllowlist возвращает middleware, пропускающее запрос только если IP клиента
// входит в одну из разрешённых подсетей. Используется для вебхуков платёжного
// шлюза (ResultURL Robokassa), которые должны приходить только с его адресов.
// Если список пуст, middleware пропускает все запросы (фильтр отключён) — это
// удобно для dev и не ломает работу, пока подсети не сконфигурированы.
func IPAllowlist(allowed []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if len(allowed) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := net.ParseIP(clientIP(r))
			if ip != nil {
				for _, n := range allowed {
					if n.Contains(ip) {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			respondError(w, r, http.StatusForbidden, "доступ запрещён")
		})
	}
}

// CORSOptions настраивает поведение CORS-middleware.
type CORSOptions struct {
	// AllowedOrigins — список разрешённых Origin. Спецзначение "*" разрешает любой источник.
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int // секунды кеширования preflight-ответа
}

// DefaultCORSOptions возвращает безопасные значения по умолчанию для API заказов.
func DefaultCORSOptions(origins []string) CORSOptions {
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	return CORSOptions{
		AllowedOrigins: origins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Content-Type", "X-Access-Token"},
		MaxAge:         300,
	}
}

// CORS возвращает middleware, выставляющее заголовки CORS и обрабатывающее preflight (OPTIONS).
func CORS(opts CORSOptions) func(http.Handler) http.Handler {
	allowMethods := strings.Join(opts.AllowedMethods, ", ")
	allowHeaders := strings.Join(opts.AllowedHeaders, ", ")
	maxAge := opts.MaxAge

	allowAll := false
	for _, o := range opts.AllowedOrigins {
		if o == "*" {
			allowAll = true
			break
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (allowAll || originAllowed(origin, opts.AllowedOrigins)) {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					// Origin влияет на ответ — помечаем для корректного кеширования.
					w.Header().Add("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Methods", allowMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
				if maxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", itoa(maxAge))
				}
			}

			// Preflight-запрос завершаем сразу, без передачи дальше по цепочке.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if strings.EqualFold(a, origin) {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// MaxBodyBytes возвращает middleware, ограничивающее размер тела входящего
// запроса. Без него клиент может прислать сколь угодно большое тело и исчерпать
// память сервиса. При превышении лимита чтение тела вернёт ошибку, а http-сервер
// ответит 413 Request Entity Too Large.
// n <= 0 отключает ограничение.
func MaxBodyBytes(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if n > 0 && r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ipRateLimiter ограничивает частоту запросов по IP клиента, используя
// токен-бакет на каждый IP. Старые записи периодически вычищаются, чтобы
// мапа не росла бесконечно.
type ipRateLimiter struct {
	mu       sync.Mutex
	clients  map[string]*clientLimiter
	rate     rate.Limit
	burst    int
	ttl      time.Duration
	lastSwep time.Time
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter возвращает middleware, ограничивающее число запросов с одного IP.
// rps — допустимое число запросов в секунду, burst — размер всплеска.
func RateLimiter(rps float64, burst int) func(http.Handler) http.Handler {
	rl := &ipRateLimiter{
		clients:  make(map[string]*clientLimiter),
		rate:     rate.Limit(rps),
		burst:    burst,
		ttl:      3 * time.Minute,
		lastSwep: time.Now(),
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.allow(clientIP(r)) {
				w.Header().Set("Retry-After", "1")
				respondError(w, r, http.StatusTooManyRequests, "слишком много запросов")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.Sub(rl.lastSwep) > rl.ttl {
		for k, v := range rl.clients {
			if now.Sub(v.lastSeen) > rl.ttl {
				delete(rl.clients, k)
			}
		}
		rl.lastSwep = now
	}

	cl, ok := rl.clients[ip]
	if !ok {
		cl = &clientLimiter{limiter: rate.NewLimiter(rl.rate, rl.burst)}
		rl.clients[ip] = cl
	}
	cl.lastSeen = now
	return cl.limiter.Allow()
}

// clientIP извлекает IP клиента из запроса, учитывая, что RealIP middleware
// мог уже подставить адрес в RemoteAddr.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
