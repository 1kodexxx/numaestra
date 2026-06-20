package apphttp

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// AdminAuth возвращает middleware, проверяющий Bearer-токен в заголовке Authorization.
// Все маршруты под этим middleware доступны только при совпадении токена с token.
// Сравнение выполняется за константное время через subtle.ConstantTimeCompare
// для защиты от timing-атак.
func AdminAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				respondError(w, r, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
