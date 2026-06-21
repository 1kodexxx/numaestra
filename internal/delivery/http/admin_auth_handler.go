package apphttp

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/numaestra/numaestra/pkg/adminsession"
)

// adminSessionTTL — срок действия сессии админки после входа по логину/паролю.
const adminSessionTTL = 12 * time.Hour

// AdminAuthHandler обрабатывает вход/выход и проверку текущей сессии админки
// для фронтенда /admin. Использует подписанные stateless-токены (см. pkg/adminsession) —
// сравнение логина/пароля и подписи токена выполняется за константное время.
type AdminAuthHandler struct {
	login         string
	password      string
	sessionSecret []byte
	// secureCookie выставляет флаг Secure у cookie. Должен быть true везде,
	// кроме локальной dev-разработки по http (иначе браузер не примет cookie за TLS).
	secureCookie bool
	log          *slog.Logger
	// rdb — опциональный Redis-клиент для распределённого rate-limit на /login
	// (защита от перебора пароля). Без него используется per-instance лимитер.
	rdb *redis.Client
}

func NewAdminAuthHandler(login, password string, sessionSecret []byte, secureCookie bool, log *slog.Logger) *AdminAuthHandler {
	return &AdminAuthHandler{
		login:         login,
		password:      password,
		sessionSecret: sessionSecret,
		secureCookie:  secureCookie,
		log:           log,
	}
}

// WithRedis подключает распределённый rate-limit для /login (на всех инстансах
// общий лимит попыток на IP, а не per-instance).
func (h *AdminAuthHandler) WithRedis(rdb *redis.Client) *AdminAuthHandler {
	h.rdb = rdb
	return h
}

// Routes возвращает публичные маршруты входа/выхода (без AdminAuth — иначе
// зайти было бы нечем). /login защищён жёстким rate-limit от перебора пароля.
func (h *AdminAuthHandler) Routes() chi.Router {
	r := chi.NewRouter()

	var loginLimiter func(http.Handler) http.Handler
	if h.rdb != nil {
		loginLimiter = DistributedRateLimiter(h.rdb, 5, time.Minute)
	} else {
		loginLimiter = RateLimiter(0.1, 5)
	}

	r.With(loginLimiter).Post("/login", h.Login)
	r.Post("/logout", h.Logout)
	return r
}

type adminLoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Login проверяет логин/пароль и при успехе выставляет httpOnly-cookie сессии.
// POST /api/v1/admin/login
func (h *AdminAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req adminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	// Пустые ADMIN_LOGIN/ADMIN_PASSWORD в конфиге означают "вход отключён" —
	// без этой проверки пустые учётные данные на входе совпали бы с пустыми
	// в конфиге и пропустили бы любого.
	if h.login == "" || h.password == "" ||
		!constantTimeEqual(req.Login, h.login) || !constantTimeEqual(req.Password, h.password) {
		h.log.Warn("admin: неудачная попытка входа", "ip", clientIP(r))
		respondError(w, r, http.StatusUnauthorized, "неверный логин или пароль")
		return
	}

	token := adminsession.Issue(h.sessionSecret, req.Login, adminSessionTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     AdminSessionCookieName,
		Value:    token,
		Path:     "/api/v1/admin",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(adminSessionTTL.Seconds()),
	})
	h.log.Info("admin: успешный вход", "login", req.Login, "ip", clientIP(r))
	respondJSON(w, http.StatusOK, map[string]string{"login": req.Login})
}

// Logout очищает cookie сессии. POST /api/v1/admin/logout
func (h *AdminAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     AdminSessionCookieName,
		Value:    "",
		Path:     "/api/v1/admin",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

// Me возвращает логин текущей сессии. Маршрут защищён AdminAuth — фронтенд
// дёргает его при загрузке /admin, чтобы понять, нужен ли редирект на /admin/login.
// GET /api/v1/admin/me
func (h *AdminAuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	login := h.login
	if cookie, err := r.Cookie(AdminSessionCookieName); err == nil {
		if l, ok := adminsession.Verify(h.sessionSecret, cookie.Value); ok {
			login = l
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{"login": login})
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
