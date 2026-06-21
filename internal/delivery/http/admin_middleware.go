package apphttp

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/numaestra/numaestra/pkg/adminsession"
)

// AdminSessionCookieName — имя cookie сессии админки, выдаваемой LoginHandler.
const AdminSessionCookieName = "admin_session"

// AdminAuth возвращает middleware, защищающий /api/v1/admin/*. Принимает один
// из двух способов аутентификации:
//  1. Bearer-токен в заголовке Authorization (ADMIN_TOKEN) — для скриптов/CI/curl;
//  2. подписанная cookie-сессия (см. AdminSessionCookieName), выданная
//     POST /api/v1/admin/login по логину/паролю — для фронтенда /admin.
//
// Bearer-токен сравнивается за константное время (subtle.ConstantTimeCompare),
// cookie-сессия проверяется через adminsession.Verify (HMAC, тоже константное время).
func AdminAuth(token string, sessionSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); bearer != "" && token != "" &&
				subtle.ConstantTimeCompare([]byte(bearer), []byte(token)) == 1 {
				next.ServeHTTP(w, r)
				return
			}

			if cookie, err := r.Cookie(AdminSessionCookieName); err == nil {
				if _, ok := adminsession.Verify(sessionSecret, cookie.Value); ok {
					next.ServeHTTP(w, r)
					return
				}
			}

			respondError(w, r, http.StatusUnauthorized, "unauthorized")
		})
	}
}
