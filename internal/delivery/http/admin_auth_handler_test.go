package apphttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/numaestra/numaestra/pkg/adminsession"
)

var testAdminSessionSecret = []byte("01234567890123456789012345678901")

func newTestAdminAuthHandler() *AdminAuthHandler {
	return NewAdminAuthHandler("owner", "correct-password", testAdminSessionSecret, false, discardAdminLogger())
}

func TestAdminAuthHandler_Login_Success(t *testing.T) {
	h := newTestAdminAuthHandler()
	router := h.Routes()

	body := `{"login":"owner","password":"correct-password"}`
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", w.Code, w.Body.String())
	}

	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != AdminSessionCookieName {
		t.Fatalf("ожидали cookie %q, получили %+v", AdminSessionCookieName, cookies)
	}
	if !cookies[0].HttpOnly {
		t.Error("cookie сессии должна быть HttpOnly")
	}
	if cookies[0].SameSite != http.SameSiteStrictMode {
		t.Error("cookie сессии должна быть SameSite=Strict")
	}

	login, ok := adminsession.Verify(testAdminSessionSecret, cookies[0].Value)
	if !ok || login != "owner" {
		t.Errorf("токен в cookie должен быть валиден с login=owner, получили login=%q ok=%v", login, ok)
	}
}

func TestAdminAuthHandler_Login_WrongPassword(t *testing.T) {
	h := newTestAdminAuthHandler()
	router := h.Routes()

	body := `{"login":"owner","password":"неверный"}`
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ожидали 401, получили %d", w.Code)
	}
	if len(w.Result().Cookies()) != 0 {
		t.Error("при неудачном входе cookie не должна выставляться")
	}
}

func TestAdminAuthHandler_Login_WrongLogin(t *testing.T) {
	h := newTestAdminAuthHandler()
	router := h.Routes()

	body := `{"login":"чужой","password":"correct-password"}`
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ожидали 401, получили %d", w.Code)
	}
}

func TestAdminAuthHandler_Login_EmptyCredentialsConfigured_AlwaysRejects(t *testing.T) {
	h := NewAdminAuthHandler("", "", testAdminSessionSecret, false, discardAdminLogger())
	router := h.Routes()

	// Пустые логин/пароль в запросе не должны совпасть с пустыми в конфиге.
	body := `{"login":"","password":""}`
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ожидали 401 при незаданных учётных данных, получили %d", w.Code)
	}
}

func TestAdminAuthHandler_Login_InvalidJSON(t *testing.T) {
	h := newTestAdminAuthHandler()
	router := h.Routes()

	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("{не json"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
}

func TestAdminAuthHandler_Login_RateLimited(t *testing.T) {
	h := newTestAdminAuthHandler()
	router := h.Routes()

	body := `{"login":"owner","password":"неверный"}`
	var lastCode int
	for i := 0; i < 10; i++ {
		r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		lastCode = w.Code
		if lastCode == http.StatusTooManyRequests {
			return
		}
	}
	t.Fatalf("ожидали 429 после серии неудачных попыток входа, последний код: %d", lastCode)
}

func TestAdminAuthHandler_Logout_ClearsCookie(t *testing.T) {
	h := newTestAdminAuthHandler()
	router := h.Routes()

	r := httptest.NewRequest(http.MethodPost, "/logout", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("logout должен выставлять cookie с MaxAge < 0 (удаление), получили %+v", cookies)
	}
}

func TestAdminAuthHandler_Me_WithValidCookie(t *testing.T) {
	h := newTestAdminAuthHandler()

	r := httptest.NewRequest(http.MethodGet, "/me", nil)
	r.AddCookie(&http.Cookie{Name: AdminSessionCookieName, Value: adminsession.Issue(testAdminSessionSecret, "owner", time.Hour)})
	w := httptest.NewRecorder()
	h.Me(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "owner") {
		t.Errorf("ответ должен содержать login=owner, получили %s", w.Body.String())
	}
}
