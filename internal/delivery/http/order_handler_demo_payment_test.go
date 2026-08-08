package apphttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/pkg/notify"
	"github.com/numaestra/numaestra/pkg/robokassa"
)

// Цены сценария платного демо на HTTP-слое: заказ 990 ₽ = демо 50 ₽ + 940 ₽.
const (
	hDemoPrice     int64 = 5000
	hOrderPrice    int64 = 99000
	hRemainingSum  int64 = hOrderPrice - hDemoPrice
	hDemoPriceRubs       = "50.00"
)

// newDemoPaymentHandler — обработчик с ценой заказа 990 ₽ и платным демо 50 ₽.
func newDemoPaymentHandler(t *testing.T) (*OrderHandler, http.Handler, *hOrderRepo) {
	t.Helper()
	repo := newHOrderRepo()
	uc := usecase.NewOrderUseCase(repo, nil, &hQueue{}, nil, nil, notify.NewLogNotifier(discardLogger()), nil,
		usecase.NewNoopPromptUseCase(), hOrderPrice, hTxManager{}, discardLogger()).
		WithDemoPrice(hDemoPrice)
	rk := robokassa.New(hMerchant, hPass1, hPass2, "", true)
	h := NewOrderHandler(uc, discardLogger(), rk, nil)
	return h, h.Routes(), repo
}

// postWebhook отправляет вебхук Robokassa с валидной подписью для указанной суммы.
func postWebhook(t *testing.T, router http.Handler, invoiceID int64, outSum string) *httptest.ResponseRecorder {
	t.Helper()
	invID := fmt.Sprintf("%d", invoiceID)
	form := url.Values{}
	form.Set("OutSum", outSum)
	form.Set("InvId", invID)
	form.Set("SignatureValue", webhookSig(outSum, invID))

	req := httptest.NewRequest(http.MethodPost, "/webhook/robokassa", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateOrder_ОтдаётОбеПлатёжныеСсылки(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(validOrderBody("")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d (%s)", rec.Code, rec.Body.String())
	}

	var resp OrderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if resp.DemoInvoiceID == 0 || resp.DemoInvoiceID == resp.InvoiceID {
		t.Errorf("demo_invoice_id = %d, ожидался отдельный ненулевой InvId (основной %d)", resp.DemoInvoiceID, resp.InvoiceID)
	}
	if resp.DemoAmountKopecks != hDemoPrice {
		t.Errorf("demo_amount_kopecks = %d, ожидалось %d", resp.DemoAmountKopecks, hDemoPrice)
	}
	if resp.DemoPaymentURL == "" {
		t.Error("demo_payment_url пуст — клиенту нечем оплатить первый шаг воронки")
	}
	// До оплаты демо к оплате стоит полная сумма: заказ можно закрыть одним платежом.
	if resp.RemainingKopecks != hOrderPrice {
		t.Errorf("remaining_kopecks = %d, ожидалось %d", resp.RemainingKopecks, hOrderPrice)
	}
	_ = h
}

func TestWebhook_ОплатаДемоНеОплачиваетЗаказ(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	rec := postWebhook(t, router, order.DemoInvoiceID(), hDemoPriceRubs)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	want := fmt.Sprintf("OK%d", order.DemoInvoiceID())
	if rec.Body.String() != want {
		t.Errorf("Robokassa ожидает %q, получили %q", want, rec.Body.String())
	}

	stored, err := h.uc.GetOrder(t.Context(), order.ID())
	if err != nil {
		t.Fatalf("получение заказа: %v", err)
	}
	if !stored.DemoPaid() {
		t.Error("демо должно быть помечено оплаченным")
	}
	if stored.PaymentStatus() != domain.PaymentStatusPending {
		t.Errorf("статус оплаты заказа = %q, ожидался pending: 50 ₽ не оплачивают песню", stored.PaymentStatus())
	}
	if got := stored.RemainingKopecks(); got != hRemainingSum {
		t.Errorf("остаток к доплате = %d, ожидалось %d", got, hRemainingSum)
	}
}

// Повторная доставка вебхука по оплаченному демо — успех, а не ошибка: иначе
// Robokassa уйдёт в бесконечные ретраи.
func TestWebhook_ПовторнаяОплатаДемоИдемпотентна(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	for i := 0; i < 2; i++ {
		rec := postWebhook(t, router, order.DemoInvoiceID(), hDemoPriceRubs)
		if rec.Code != http.StatusOK {
			t.Fatalf("доставка №%d: ожидали 200, получили %d (%s)", i+1, rec.Code, rec.Body.String())
		}
	}
}

func TestWebhook_ЧужаяСуммаЗаДемоОтклоняется(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	rec := postWebhook(t, router, order.DemoInvoiceID(), "1.00")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 при несовпадении суммы демо, получили %d (%s)", rec.Code, rec.Body.String())
	}

	stored, _ := h.uc.GetOrder(t.Context(), order.ID())
	if stored.DemoPaid() {
		t.Error("демо не должно считаться оплаченным при неверной сумме")
	}
}

// Полный проход воронки: 50 ₽ за демо, затем 940 ₽ за песню.
func TestWebhook_ДоплатаПослеДемоОплачиваетЗаказ(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	if rec := postWebhook(t, router, order.DemoInvoiceID(), hDemoPriceRubs); rec.Code != http.StatusOK {
		t.Fatalf("оплата демо: %d (%s)", rec.Code, rec.Body.String())
	}

	// Полная сумма после зачёта демо больше не принимается.
	if rec := postWebhook(t, router, order.InvoiceID(), robokassa.FormatAmount(hOrderPrice)); rec.Code != http.StatusBadRequest {
		t.Errorf("полная сумма должна отклоняться после зачёта демо, получили %d", rec.Code)
	}

	rec := postWebhook(t, router, order.InvoiceID(), robokassa.FormatAmount(hRemainingSum))
	if rec.Code != http.StatusOK {
		t.Fatalf("доплата остатка: ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}

	stored, _ := h.uc.GetOrder(t.Context(), order.ID())
	if stored.PaymentStatus() != domain.PaymentStatusPaid {
		t.Errorf("статус оплаты = %q, ожидался paid", stored.PaymentStatus())
	}
}

// Публичный статус отдаёт суммы, но НЕ платёжные ссылки: они содержат email, а
// эндпоинт открыт по одному лишь UUID заказа.
func TestPublicStatus_НеРаскрываетПлатёжныеСсылки(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	req := httptest.NewRequest(http.MethodGet, "/"+order.ID().String()+"/status", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	for _, key := range []string{"payment_url", "demo_payment_url"} {
		if _, ok := body[key]; ok {
			t.Errorf("публичный статус не должен содержать %q — ссылка раскрывает email клиента", key)
		}
	}
	if got := body["demo_amount_kopecks"]; got != float64(hDemoPrice) {
		t.Errorf("demo_amount_kopecks = %v, ожидалось %d", got, hDemoPrice)
	}
	if got := body["demo_payment_status"]; got != string(domain.PaymentStatusPending) {
		t.Errorf("demo_payment_status = %v, ожидался pending", got)
	}
}

// --- GET /{id}/demo-payment-url ---

// getDemoPaymentURL — запрос ссылки на оплату демо с токеном доступа заказа.
func getDemoPaymentURL(router http.Handler, orderID, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/"+orderID+"/demo-payment-url", nil)
	if token != "" {
		req.Header.Set("X-Access-Token", token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestGetDemoPaymentURL_ОтдаётСсылкуНаОплатуДемо(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	rec := getDemoPaymentURL(router, order.ID().String(), order.AccessToken())
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if body["payment_url"] == "" {
		t.Error("payment_url пуст — клиенту нечем оплатить демо")
	}
	// Ссылка должна вести на счёт ДЕМО, а не на основной.
	if !strings.Contains(body["payment_url"], fmt.Sprintf("InvId=%d", order.DemoInvoiceID())) {
		t.Errorf("ссылка ведёт не на счёт демо (InvId=%d): %s", order.DemoInvoiceID(), body["payment_url"])
	}
}

func TestGetDemoPaymentURL_ТребуетТокенДоступа(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	rec := getDemoPaymentURL(router, order.ID().String(), "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("без токена ожидали 401, получили %d", rec.Code)
	}
}

func TestGetDemoPaymentURL_ОплаченноеДемоОтдаётConflict(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	if err := h.uc.HandleDemoPaymentSuccess(t.Context(), order.DemoInvoiceID(), hDemoPrice); err != nil {
		t.Fatalf("оплата демо: %v", err)
	}

	rec := getDemoPaymentURL(router, order.ID().String(), order.AccessToken())
	if rec.Code != http.StatusConflict {
		t.Fatalf("для оплаченного демо ожидали 409, получили %d (%s)", rec.Code, rec.Body.String())
	}
}

// Заказ без второго счёта (легаси / бесплатное демо) платить за демо не может.
func TestGetDemoPaymentURL_БезСчётаНаДемоОтдаётConflict(t *testing.T) {
	h, router, _ := newTestHandler(t) // без WithDemoPrice
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	rec := getDemoPaymentURL(router, order.ID().String(), order.AccessToken())
	if rec.Code != http.StatusConflict {
		t.Fatalf("ожидали 409, получили %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestGetDemoPaymentURL_НекорректныйIDОтдаёт400(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	rec := getDemoPaymentURL(router, "не-uuid", order.AccessToken())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", rec.Code)
	}
}

// Ссылка на доплату за песню выставляется на ОСТАТОК: после зачёта демо в ней
// должна стоять разница, иначе клиент заплатит 50 ₽ дважды.
func TestGetPaymentURL_ПослеОплатыДемоВыставленНаОстаток(t *testing.T) {
	h, router, _ := newDemoPaymentHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	if err := h.uc.HandleDemoPaymentSuccess(t.Context(), order.DemoInvoiceID(), hDemoPrice); err != nil {
		t.Fatalf("оплата демо: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/"+order.ID().String()+"/payment-url", nil)
	req.Header.Set("X-Access-Token", order.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("разбор ответа: %v", err)
	}
	if !strings.Contains(body["payment_url"], "OutSum=940") {
		t.Errorf("ожидали счёт на остаток 940 ₽, получили: %s", body["payment_url"])
	}
}
