package robokassa

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Тестовые учётные данные (не боевые).
const (
	testMerchantLogin = "TestMerchant"
	testPassword1     = "TestPassword1"
	testPassword2     = "TestPassword2"
)

func newTestClient(isTest bool) *Client {
	return New(testMerchantLogin, testPassword1, testPassword2, isTest)
}

// --- FormatAmount ---

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		kopecks int64
		want    string
	}{
		{150000, "1500.00"},       // ровные рубли
		{150050, "1500.50"},       // с копейками
		{1, "0.01"},               // одна копейка
		{100, "1.00"},             // один рубль
		{99999, "999.99"},         // нестандартная сумма
		{100000000, "1000000.00"}, // большая сумма
	}

	for _, tt := range tests {
		got := FormatAmount(tt.kopecks)
		if got != tt.want {
			t.Errorf("FormatAmount(%d) = %q, хотели %q", tt.kopecks, got, tt.want)
		}
	}
}

// --- signPayment (через PaymentURL) ---

func TestClient_PaymentURL_ContainsRequiredParams(t *testing.T) {
	c := newTestClient(false)
	u := c.PaymentURL("1500.00", 42, "Тестовый заказ")

	for _, param := range []string{"MerchantLogin", "OutSum", "InvId", "SignatureValue", "Description"} {
		if !strings.Contains(u, param) {
			t.Errorf("PaymentURL не содержит параметр %q: %s", param, u)
		}
	}
}

func TestClient_PaymentURL_IsTestFlag(t *testing.T) {
	prod := newTestClient(false)
	test := newTestClient(true)

	uProd := prod.PaymentURL("100.00", 1, "Desc")
	uTest := test.PaymentURL("100.00", 1, "Desc")

	if !strings.Contains(uProd, "IsTest=0") {
		t.Errorf("продакшн URL должен содержать IsTest=0: %s", uProd)
	}
	if !strings.Contains(uTest, "IsTest=1") {
		t.Errorf("тестовый URL должен содержать IsTest=1: %s", uTest)
	}
}

func TestClient_PaymentURL_SignatureIsUpperMD5(t *testing.T) {
	c := newTestClient(false)
	u := c.PaymentURL("1500.00", 7, "Desc")

	// Извлекаем SignatureValue из URL
	sig := extractParam(u, "SignatureValue")
	if sig == "" {
		t.Fatal("SignatureValue не найден в URL")
	}
	// MD5 в hex = 32 символа, верхний регистр
	if len(sig) != 32 {
		t.Errorf("SignatureValue должен быть 32 символа, получили %d: %q", len(sig), sig)
	}
	if sig != strings.ToUpper(sig) {
		t.Errorf("SignatureValue должен быть в верхнем регистре, получили: %q", sig)
	}
}

// --- VerifyWebhook ---

func TestClient_VerifyWebhook_ValidSignature(t *testing.T) {
	c := newTestClient(false)

	outSum := "1500.00"
	invID := "42"

	// Правильная формула: OutSum:InvId:Password2
	correctSig := c.signWebhook(outSum, invID)
	if !c.VerifyWebhook(outSum, invID, correctSig) {
		t.Error("VerifyWebhook должен принять корректную подпись")
	}
}

func TestClient_VerifyWebhook_InvalidSignature(t *testing.T) {
	c := newTestClient(false)
	if c.VerifyWebhook("1500.00", "42", "DEADBEEF00000000DEADBEEF00000000") {
		t.Error("VerifyWebhook должен отклонить неверную подпись")
	}
}

func TestClient_VerifyWebhook_CaseInsensitive(t *testing.T) {
	c := newTestClient(false)
	sig := c.signWebhook("500.00", "1")

	// Robokassa может прислать подпись в любом регистре
	if !c.VerifyWebhook("500.00", "1", strings.ToLower(sig)) {
		t.Error("VerifyWebhook должен принять подпись в нижнем регистре")
	}
	if !c.VerifyWebhook("500.00", "1", strings.ToUpper(sig)) {
		t.Error("VerifyWebhook должен принять подпись в верхнем регистре")
	}
}

func TestClient_VerifyWebhook_WrongPassword(t *testing.T) {
	c := newTestClient(false)
	// Подпись посчитана с Password1, а проверяем с Password2 — должна упасть
	wrongSig := c.signPayment("1500.00", "42")
	if c.VerifyWebhook("1500.00", "42", wrongSig) {
		t.Error("подпись с неверным паролем не должна проходить верификацию")
	}
}

func TestClient_VerifyWebhook_DifferentAmounts(t *testing.T) {
	c := newTestClient(false)
	sig := c.signWebhook("1500.00", "42")
	// Подменяем сумму
	if c.VerifyWebhook("1500.01", "42", sig) {
		t.Error("подпись не должна проходить при изменённой сумме")
	}
}

// --- Согласованность signPayment / PaymentURL ---

func TestClient_PaymentAndWebhook_SameAmount(t *testing.T) {
	// Критический тест: сумма в ссылке оплаты и в верификации вебхука
	// должна быть в одном формате, иначе подписи не совпадут.
	c := newTestClient(false)
	kopecks := int64(150050) // 1500 рублей 50 копеек

	outSum := FormatAmount(kopecks)
	if outSum != "1500.50" {
		t.Fatalf("FormatAmount вернул %q вместо '1500.50'", outSum)
	}

	// Имитируем: сформировали ссылку → Robokassa прислала вебхук с той же суммой
	invID := "99"
	paymentSig := extractParam(c.PaymentURL(outSum, 99, "Desc"), "SignatureValue")
	webhookSig := c.signWebhook(outSum, invID)

	// Подписи разные (разные пароли), но формат суммы должен быть одинаковым
	// Проверяем что вебхук с той же суммой проходит верификацию
	if !c.VerifyWebhook(outSum, invID, webhookSig) {
		t.Error("вебхук с суммой из PaymentURL должен проходить верификацию")
	}
	_ = paymentSig // подпись оплаты использует Password1, вебхука — Password2
}

// --- Refund ---

// newTestRefundClient создаёт Client с httptest-сервером вместо prod URL.
func newTestRefundClient(srv *httptest.Server) *Client {
	c := New(testMerchantLogin, testPassword1, testPassword2, true)
	c.httpClient = srv.Client()
	c.refundURL = srv.URL + "/refund"
	return c
}

func TestRefund_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("ожидали POST, получили %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("неверный Content-Type: %s", ct)
		}
		fmt.Fprint(w, "OK42")
	}))
	defer srv.Close()

	c := newTestRefundClient(srv)
	if err := c.Refund(context.Background(), "1500.00", 42); err != nil {
		t.Fatalf("ожидали успех, получили: %v", err)
	}
}

func TestRefund_ServerReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ERROR5")
	}))
	defer srv.Close()

	c := newTestRefundClient(srv)
	err := c.Refund(context.Background(), "1500.00", 42)
	if err == nil {
		t.Fatal("ожидали ошибку при ответе ERROR от сервера")
	}
	if !strings.Contains(err.Error(), "ERROR5") {
		t.Errorf("ошибка должна содержать тело ответа, получили: %v", err)
	}
}

func TestRefund_HTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestRefundClient(srv)
	err := c.Refund(context.Background(), "1500.00", 42)
	if err == nil {
		t.Fatal("ожидали ошибку при HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("ошибка должна содержать HTTP-статус, получили: %v", err)
	}
}

func TestRefund_NetworkError(t *testing.T) {
	// Сервер немедленно закрывается — имитирует сетевую ошибку.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	c := newTestRefundClient(srv)
	if err := c.Refund(context.Background(), "1500.00", 42); err == nil {
		t.Fatal("ожидали ошибку при недоступном сервере")
	}
}

func TestRefund_SendsCorrectParams(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotBody = r.Form.Encode()
		fmt.Fprint(w, "OK99")
	}))
	defer srv.Close()

	c := newTestRefundClient(srv)
	_ = c.Refund(context.Background(), "750.00", 99)

	for _, param := range []string{"MrchLogin", "InvId", "OutSum", "Signature"} {
		if !strings.Contains(gotBody, param) {
			t.Errorf("параметр %q отсутствует в теле запроса: %s", param, gotBody)
		}
	}
}

// --- helpers ---

func extractParam(rawURL, key string) string {
	// Простой парсер query-параметров для тестов
	parts := strings.SplitN(rawURL, "?", 2)
	if len(parts) < 2 {
		return ""
	}
	for _, pair := range strings.Split(parts[1], "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 && kv[0] == key {
			return kv[1]
		}
	}
	return ""
}
