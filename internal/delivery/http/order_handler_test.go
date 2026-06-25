package apphttp

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/pkg/robokassa"
)

const (
	hMerchant = "TestMerchant"
	hPass1    = "pass1"
	hPass2    = "pass2"
)

func newTestHandler(t *testing.T) (*OrderHandler, http.Handler, *hOrderRepo) {
	t.Helper()
	repo := newHOrderRepo()
	uc := usecase.NewOrderUseCase(repo, nil, &hQueue{}, nil, nil, nil, nil, usecase.NewNoopPromptUseCase(), 150000, hTxManager{}, discardLogger())
	rk := robokassa.New(hMerchant, hPass1, hPass2, true)
	h := NewOrderHandler(uc, discardLogger(), rk, nil)
	return h, h.Routes(), repo
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func webhookSig(outSum, invID string) string {
	sum := md5.Sum([]byte(fmt.Sprintf("%s:%s:%s", outSum, invID, hPass2)))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func validOrderBody(extra string) string {
	if extra != "" && !strings.HasPrefix(extra, ",") {
		extra = "," + extra
	}
	return fmt.Sprintf(`{"email":"user@example.com","brief":"Песня","consent_doc_version":%q%s}`, domain.CurrentConsentDocVersion, extra)
}

// --- CreateOrder ---

func TestHandler_CreateOrder_Success(t *testing.T) {
	_, router, _ := newTestHandler(t)

	body := validOrderBody("")
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var resp OrderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("не удалось разобрать ответ: %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("ответ должен содержать access_token (без него клиент не аутентифицируется)")
	}
	if resp.PaymentURL == "" {
		t.Error("ответ должен содержать payment_url")
	}
	if resp.AmountKopecks != 150000 {
		t.Errorf("цену определяет сервер по тарифу standard=150000, получили %d", resp.AmountKopecks)
	}
}

func TestHandler_CreateOrder_IgnoresClientAmount(t *testing.T) {
	_, router, _ := newTestHandler(t)

	// Клиент пытается занизить цену через amount_kopecks — поле игнорируется,
	// цена берётся из серверного тарифа.
	body := validOrderBody(`"amount_kopecks":1`)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp OrderResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.AmountKopecks != 150000 {
		t.Errorf("клиентская сумma должна игнорироваться, ожидали 150000, получили %d", resp.AmountKopecks)
	}
}

func TestHandler_CreateOrder_InvalidJSON(t *testing.T) {
	_, router, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{не json"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", rec.Code)
	}
}

func TestHandler_CreateOrder_BriefTooLong(t *testing.T) {
	_, router, _ := newTestHandler(t)
	longBrief := strings.Repeat("я", domain.MaxBriefLength+1)
	body := fmt.Sprintf(`{"email":"user@example.com","brief":%q,"consent_doc_version":%q}`, longBrief, domain.CurrentConsentDocVersion)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 при слишком длинном brief, получили %d", rec.Code)
	}
}

func TestHandler_ErrorResponse_IncludesRequestID(t *testing.T) {
	_, routes, _ := newTestHandler(t)
	// RequestID middleware кладёт идентификатор в контекст, который хендлер
	// должен вернуть в теле ошибки.
	router := chimiddleware.RequestID(routes)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{не json"))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("не удалось разобрать тело ошибки: %v", err)
	}
	if body["error"] == "" {
		t.Error("тело ошибки должно содержать поле error")
	}
	if body["request_id"] == "" {
		t.Error("тело ошибки должно содержать request_id")
	}
}

func TestHandler_CreateOrder_MissingFields(t *testing.T) {
	_, router, _ := newTestHandler(t)
	cases := []string{
		`{"brief":"Песня"}`,        // нет контакта
		`{"email":"a@b.c"}`,        // нет brief
		`{"phone":"+79990000000"}`, // нет brief (только телефон)
	}
	for i, body := range cases {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("case %d: ожидали 400, получили %d", i, rec.Code)
		}
	}
}

func TestHandler_CreateOrder_MissingConsent(t *testing.T) {
	_, router, _ := newTestHandler(t)
	body := `{"email":"user@example.com","brief":"Песня"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d (%s)", rec.Code, rec.Body.String())
	}
}

// --- Webhook ---

func TestHandler_Webhook_Success(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	invID := fmt.Sprintf("%d", order.InvoiceID())
	outSum := robokassa.FormatAmount(150000)
	form := url.Values{}
	form.Set("OutSum", outSum)
	form.Set("InvId", invID)
	form.Set("SignatureValue", webhookSig(outSum, invID))

	req := httptest.NewRequest(http.MethodPost, "/webhook/robokassa", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "OK"+invID {
		t.Errorf("Robokassa ожидает ответ 'OK%s', получили %q", invID, rec.Body.String())
	}
}

func TestHandler_Webhook_InvalidSignature(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	invID := fmt.Sprintf("%d", order.InvoiceID())
	form := url.Values{}
	form.Set("OutSum", "1500.00")
	form.Set("InvId", invID)
	form.Set("SignatureValue", "DEADBEEF")

	req := httptest.NewRequest(http.MethodPost, "/webhook/robokassa", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 при неверной подписи, получили %d", rec.Code)
	}
}

func TestHandler_Webhook_AmountMismatch(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	invID := fmt.Sprintf("%d", order.InvoiceID())
	outSum := robokassa.FormatAmount(100000) // оплачено меньше, подпись валидна для этой суммы
	form := url.Values{}
	form.Set("OutSum", outSum)
	form.Set("InvId", invID)
	form.Set("SignatureValue", webhookSig(outSum, invID))

	req := httptest.NewRequest(http.MethodPost, "/webhook/robokassa", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 при несовпадении суммы, получили %d (%s)", rec.Code, rec.Body.String())
	}
}

// --- Protected routes ---

func TestHandler_GetOrder_RequiresToken(t *testing.T) {
	_, router, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ожидали 401 без токена, получили %d", rec.Code)
	}
}

func TestHandler_GetOrder_Success(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	req := httptest.NewRequest(http.MethodGet, "/"+order.ID().String(), nil)
	req.Header.Set("X-Access-Token", order.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var resp OrderDetailResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ID != order.ID().String() {
		t.Errorf("ожидали заказ %s, получили %s", order.ID(), resp.ID)
	}
}

func TestHandler_GetOrder_IncludesGenerationProgress(t *testing.T) {
	h, router, repo := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")
	_ = order.MarkPaid()
	_ = order.Enqueue()
	_ = order.StartProcessing(uuid.New())
	order.UpdateGenerationProgress(domain.GenerationPhaseGenerating, 55, 2)
	if err := repo.Update(context.Background(), order); err != nil {
		t.Fatalf("Update: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/"+order.ID().String(), nil)
	req.Header.Set("X-Access-Token", order.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var resp OrderDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp.GenerationPhase != string(domain.GenerationPhaseGenerating) || resp.GenerationProgress != 55 || resp.TracksReady != 2 {
		t.Errorf("прогресс в ответе: phase=%q progress=%d tracks=%d", resp.GenerationPhase, resp.GenerationProgress, resp.TracksReady)
	}
}

func TestHandler_GetOrder_TokenForDifferentOrder(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	// Несуществующий ID — 404.
	req := httptest.NewRequest(http.MethodGet, "/"+uuid.NewString(), nil)
	req.Header.Set("X-Access-Token", order.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404 для несуществующего заказа, получили %d", rec.Code)
	}
}

func TestHandler_GetOrder_SiblingOrderSameEmail(t *testing.T) {
	h, router, _ := newTestHandler(t)
	first := mustCreate(t, h, "user@example.com", "", "Первый")
	second := mustCreate(t, h, "user@example.com", "", "Второй")

	req := httptest.NewRequest(http.MethodGet, "/"+second.ID().String(), nil)
	req.Header.Set("X-Access-Token", first.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для заказа того же клиента, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var resp OrderDetailResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ID != second.ID().String() {
		t.Errorf("ожидали заказ %s, получили %s", second.ID(), resp.ID)
	}
}

func TestHandler_GetOrder_DifferentCustomerForbidden(t *testing.T) {
	h, router, _ := newTestHandler(t)
	owner := mustCreate(t, h, "owner@example.com", "", "Мой")
	other := mustCreate(t, h, "other@example.com", "", "Чужой")

	req := httptest.NewRequest(http.MethodGet, "/"+other.ID().String(), nil)
	req.Header.Set("X-Access-Token", owner.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("ожидали 403 для чужого заказа, получили %d", rec.Code)
	}
}

func TestHandler_GetPaymentURL_Success(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	req := httptest.NewRequest(http.MethodGet, "/"+order.ID().String()+"/payment-url", nil)
	req.Header.Set("X-Access-Token", order.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !strings.Contains(resp["payment_url"], "robokassa.ru") {
		t.Errorf("ожидали ссылку Robokassa, получили %q", resp["payment_url"])
	}
}

func TestHandler_GetPaymentURL_RequiresToken(t *testing.T) {
	_, router, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/"+uuid.NewString()+"/payment-url", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ожидали 401 без токена, получили %d", rec.Code)
	}
}

func TestHandler_GetPaymentURL_TokenForDifferentOrder(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	req := httptest.NewRequest(http.MethodGet, "/"+uuid.NewString()+"/payment-url", nil)
	req.Header.Set("X-Access-Token", order.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404 для несуществующего заказа, получили %d", rec.Code)
	}
}

func TestHandler_GetPaymentURL_SiblingOrderSameEmail(t *testing.T) {
	h, router, _ := newTestHandler(t)
	first := mustCreate(t, h, "user@example.com", "", "Первый")
	second := mustCreate(t, h, "user@example.com", "", "Второй")

	req := httptest.NewRequest(http.MethodGet, "/"+second.ID().String()+"/payment-url", nil)
	req.Header.Set("X-Access-Token", first.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandler_GetPublicShare_Success(t *testing.T) {
	h, router, repo := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	_ = order.MarkPaid()
	_ = order.Enqueue()
	_ = order.StartProcessing(uuid.New())
	_ = order.Complete([]domain.Track{{ID: uuid.New(), Index: 1, AudioURL: "https://cdn/a.mp3", DurationSec: 120}})
	_ = repo.Update(context.Background(), order)

	req := httptest.NewRequest(http.MethodGet, "/"+order.ID().String()+"/share", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var resp PublicShareResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Tracks) != 1 || resp.Tracks[0].AudioURL != "https://cdn/a.mp3" {
		t.Errorf("ожидали 1 трек с правильным URL, получили %+v", resp.Tracks)
	}
	if strings.Contains(rec.Body.String(), "user@example.com") || strings.Contains(rec.Body.String(), "Бриф") {
		t.Error("публичный ответ не должен содержать email или brief")
	}
}

func TestHandler_GetPublicShare_NoTokenNeeded(t *testing.T) {
	h, router, repo := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")
	_ = order.MarkPaid()
	_ = order.Enqueue()
	_ = order.StartProcessing(uuid.New())
	_ = order.Complete([]domain.Track{{ID: uuid.New(), Index: 1, AudioURL: "https://cdn/a.mp3", DurationSec: 120}})
	_ = repo.Update(context.Background(), order)

	// Никакого X-Access-Token не передаём — публичный доступ должен работать.
	req := httptest.NewRequest(http.MethodGet, "/"+order.ID().String()+"/share", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200 без токена, получили %d", rec.Code)
	}
}

func TestHandler_GetPublicShare_NotCompletedHidden(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	req := httptest.NewRequest(http.MethodGet, "/"+order.ID().String()+"/share", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("незавершённый заказ не должен быть доступен публично, получили %d", rec.Code)
	}
}

func TestHandler_GetPublicShare_NotFound(t *testing.T) {
	_, router, _ := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/"+uuid.NewString()+"/share", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404 для несуществующего заказа, получили %d", rec.Code)
	}
}

func TestHandler_ListOrders_Success(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Access-Token", order.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", rec.Code)
	}
	var resp []OrderSummaryResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Errorf("ожидали 1 заказ в списке, получили %d", len(resp))
	}
}

// --- parsePagination ---

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	limit, offset := parsePagination(r)
	if limit != 20 {
		t.Errorf("ожидали limit=20 по умолчанию, получили %d", limit)
	}
	if offset != 0 {
		t.Errorf("ожидали offset=0 по умолчанию, получили %d", offset)
	}
}

func TestParsePagination_ValidValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=50&offset=100", nil)
	limit, offset := parsePagination(r)
	if limit != 50 {
		t.Errorf("ожидали limit=50, получили %d", limit)
	}
	if offset != 100 {
		t.Errorf("ожидали offset=100, получили %d", offset)
	}
}

func TestParsePagination_LimitClampedAt100(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=999", nil)
	limit, _ := parsePagination(r)
	if limit != 100 {
		t.Errorf("limit > 100 должен обрезаться до 100, получили %d", limit)
	}
}

func TestParsePagination_ZeroLimitUsesDefault(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=0", nil)
	limit, _ := parsePagination(r)
	if limit != 20 {
		t.Errorf("limit=0 должен давать дефолт 20, получили %d", limit)
	}
}

func TestParsePagination_NegativeLimitUsesDefault(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=-5", nil)
	limit, _ := parsePagination(r)
	if limit != 20 {
		t.Errorf("отрицательный limit должен давать дефолт 20, получили %d", limit)
	}
}

func TestParsePagination_NegativeOffsetIgnored(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?offset=-10", nil)
	_, offset := parsePagination(r)
	if offset != 0 {
		t.Errorf("отрицательный offset должен игнорироваться (0), получили %d", offset)
	}
}

func TestParsePagination_NonNumericIgnored(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=abc&offset=xyz", nil)
	limit, offset := parsePagination(r)
	if limit != 20 || offset != 0 {
		t.Errorf("нечисловые значения должны давать дефолты (20, 0), получили (%d, %d)", limit, offset)
	}
}

// --- WithIdempotency ---

func TestHandler_WithIdempotency_SecondCallReturnsCached(t *testing.T) {
	repo := newHOrderRepo()
	uc := usecase.NewOrderUseCase(repo, nil, &hQueue{}, nil, nil, nil, nil, usecase.NewNoopPromptUseCase(), 150000, hTxManager{}, discardLogger())
	rk := robokassa.New(hMerchant, hPass1, hPass2, true)
	store := newFakeStore()
	h := NewOrderHandler(uc, discardLogger(), rk, nil).WithIdempotency(store)
	router := h.Routes()

	body := fmt.Sprintf(`{"email":"idem@example.com","brief":"Идемпотентный заказ","consent_doc_version":%q}`, domain.CurrentConsentDocVersion)
	const idempotencyKey = "unique-order-key-1"

	// Первый запрос — создаёт заказ, кешируется ответ.
	req1 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req1.Header.Set("Idempotency-Key", idempotencyKey)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("первый запрос: ожидали 201, получили %d (%s)", rec1.Code, rec1.Body.String())
	}
	firstBody := rec1.Body.String()

	// Второй запрос с тем же ключом — возвращает закешированный ответ.
	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req2.Header.Set("Idempotency-Key", idempotencyKey)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("второй запрос: ожидали 201, получили %d (%s)", rec2.Code, rec2.Body.String())
	}
	if rec2.Header().Get("X-Idempotent-Replayed") != "true" {
		t.Error("второй запрос должен иметь заголовок X-Idempotent-Replayed: true")
	}
	if rec2.Body.String() != firstBody {
		t.Errorf("тело повторного ответа должно совпадать с первым:\nfirst:  %s\nsecond: %s", firstBody, rec2.Body.String())
	}

	// В репозитории должен быть ровно один заказ (второй вызов не дублировал).
	if len(repo.orders) != 1 {
		t.Errorf("ожидали 1 заказ в репозитории, получили %d", len(repo.orders))
	}
}

// --- дополнительные тесты edge cases ---

func TestHandler_Webhook_MissingParams(t *testing.T) {
	_, router, _ := newTestHandler(t)

	// Нет ни одного из обязательных параметров
	req := httptest.NewRequest(http.MethodPost, "/webhook/robokassa", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 при отсутствии параметров, получили %d", rec.Code)
	}
}

func TestHandler_Webhook_InternalError(t *testing.T) {
	_, router, repo := newTestHandler(t)
	// Инъектируем ошибку в GetByInvoiceID — HandlePaymentSuccess вернёт общую ошибку → 500
	repo.getByInvErr = errors.New("db down")

	// Строим корректную подпись для вымышленного инвойса
	outSum := robokassa.FormatAmount(150000)
	invID := "9999"
	form := url.Values{}
	form.Set("OutSum", outSum)
	form.Set("InvId", invID)
	form.Set("SignatureValue", webhookSig(outSum, invID))

	req := httptest.NewRequest(http.MethodPost, "/webhook/robokassa", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("ожидали 500 при ошибке usecase, получили %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHandler_GetOrder_InvalidUUID(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	req := httptest.NewRequest(http.MethodGet, "/not-a-uuid", nil)
	req.Header.Set("X-Access-Token", order.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 для некорректного UUID в пути, получили %d", rec.Code)
	}
}

func TestHandler_ListOrders_ByPhone(t *testing.T) {
	h, router, _ := newTestHandler(t)
	// Создаём заказ только с телефоном (без email)
	order, err := h.uc.CreateOrder(context.Background(), "", "+79991234567", "Бриф", "", domain.CurrentConsentDocVersion, nil)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Access-Token", order.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var resp []OrderSummaryResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp) != 1 {
		t.Errorf("ожидали 1 заказ, получили %d", len(resp))
	}
}

// --- Webhook replay / nonce ---

func TestHandler_Webhook_ReplayProtection(t *testing.T) {
	repo := newHOrderRepo()
	uc := usecase.NewOrderUseCase(repo, nil, &hQueue{}, nil, nil, nil, nil, usecase.NewNoopPromptUseCase(), 150000, hTxManager{}, discardLogger())
	rk := robokassa.New(hMerchant, hPass1, hPass2, true)
	mini := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { rdb.Close() })
	h := NewOrderHandler(uc, discardLogger(), rk, nil).WithRedis(rdb)
	router := h.Routes()

	order, err := uc.CreateOrder(context.Background(), "user@example.com", "", "Бриф", "", domain.CurrentConsentDocVersion, nil)
	if err != nil {
		t.Fatalf("создание заказа: %v", err)
	}

	invID := fmt.Sprintf("%d", order.InvoiceID())
	outSum := robokassa.FormatAmount(150000)

	// Имитируем уже сохранённый нонс (вебхук был обработан ранее).
	rdb.Set(context.Background(), "webhook:seen:"+invID, 1, time.Hour)

	form := url.Values{}
	form.Set("OutSum", outSum)
	form.Set("InvId", invID)
	form.Set("SignatureValue", webhookSig(outSum, invID))

	req := httptest.NewRequest(http.MethodPost, "/webhook/robokassa", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("replay должен вернуть 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "OK"+invID {
		t.Errorf("ожидали 'OK%s', получили %q", invID, rec.Body.String())
	}
	// Заказ должен оставаться pending — replay-путь не запускает HandlePaymentSuccess.
	got, _ := repo.GetByInvoiceID(context.Background(), order.InvoiceID())
	if got.PaymentStatus() != domain.PaymentStatusPending {
		t.Error("replay не должен изменять статус оплаты заказа")
	}
}

func TestHandler_Webhook_PaymentWindowExpired(t *testing.T) {
	h, router, repo := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф")

	// Вручную сдвигаем CreatedAt заказа в прошлое — за пределы 72-часового окна.
	repo.mu.Lock()
	snap := repo.orders[order.ID()]
	snap.CreatedAt = time.Now().Add(-73 * time.Hour)
	repo.orders[order.ID()] = snap
	repo.mu.Unlock()

	invID := fmt.Sprintf("%d", order.InvoiceID())
	outSum := robokassa.FormatAmount(150000)
	form := url.Values{}
	form.Set("OutSum", outSum)
	form.Set("InvId", invID)
	form.Set("SignatureValue", webhookSig(outSum, invID))

	req := httptest.NewRequest(http.MethodPost, "/webhook/robokassa", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 при истёкшем платёжном окне, получили %d (%s)", rec.Code, rec.Body.String())
	}
}

// --- helpers ---

func mustCreate(t *testing.T, h *OrderHandler, email, phone, brief string) *domain.Order {
	t.Helper()
	order, err := h.uc.CreateOrder(context.Background(), email, phone, brief, "", domain.CurrentConsentDocVersion, nil)
	if err != nil {
		t.Fatalf("подготовка заказа: %v", err)
	}
	return order
}

// --- минимальные in-memory моки для конструирования use case ---

type hOrderRepo struct {
	mu          sync.Mutex
	orders      map[uuid.UUID]domain.OrderSnapshot
	byInvoice   map[int64]uuid.UUID
	seq         int64
	getByInvErr error // инъекция ошибки для GetByInvoiceID → webhook 500
}

func newHOrderRepo() *hOrderRepo {
	return &hOrderRepo{
		orders:    make(map[uuid.UUID]domain.OrderSnapshot),
		byInvoice: make(map[int64]uuid.UUID),
	}
}

func (r *hOrderRepo) Create(_ context.Context, o *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := o.Snapshot()
	r.orders[snap.ID] = snap
	r.byInvoice[snap.InvoiceID] = snap.ID
	return nil
}

func (r *hOrderRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return domain.RestoreOrder(snap), nil
}

func (r *hOrderRepo) GetByInvoiceID(_ context.Context, invoiceID int64) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getByInvErr != nil {
		return nil, r.getByInvErr
	}
	id, ok := r.byInvoice[invoiceID]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return domain.RestoreOrder(r.orders[id]), nil
}

func (r *hOrderRepo) SetAdminFeedback(_ context.Context, id uuid.UUID, feedback string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	snap.AdminFeedback = feedback
	snap.AdminFeedbackAt = &at
	r.orders[id] = snap
	return nil
}

func (r *hOrderRepo) Update(_ context.Context, o *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := o.Snapshot()
	r.orders[snap.ID] = snap
	r.byInvoice[snap.InvoiceID] = snap.ID
	return nil
}

func (r *hOrderRepo) ApplyPaymentSuccess(_ context.Context, o *domain.Order) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.orders[o.ID()]
	if !ok {
		return false, domain.ErrOrderNotFound
	}
	if cur.PaymentStatus != domain.PaymentStatusPending {
		return false, nil
	}
	snap := o.Snapshot()
	r.orders[snap.ID] = snap
	return true, nil
}

func (r *hOrderRepo) ListByCustomerEmail(_ context.Context, email string, _, _ int) ([]*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Order
	for _, snap := range r.orders {
		if snap.CustomerEmail == email {
			out = append(out, domain.RestoreOrder(snap))
		}
	}
	return out, nil
}

func (r *hOrderRepo) ListByCustomerPhone(_ context.Context, phone string, _, _ int) ([]*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Order
	for _, snap := range r.orders {
		if snap.CustomerPhone == phone {
			out = append(out, domain.RestoreOrder(snap))
		}
	}
	return out, nil
}

func (r *hOrderRepo) NextInvoiceID(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return r.seq, nil
}

func (r *hOrderRepo) GetByAccessToken(_ context.Context, token string) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, snap := range r.orders {
		if snap.AccessToken == token {
			return domain.RestoreOrder(snap), nil
		}
	}
	return nil, domain.ErrOrderNotFound
}

func (r *hOrderRepo) ListAll(_ context.Context, _, _ int) ([]*domain.Order, error) {
	return nil, nil
}

func (r *hOrderRepo) CountAll(_ context.Context) (int, error) { return 0, nil }
func (r *hOrderRepo) ListStuckProcessing(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	return nil, nil
}
func (r *hOrderRepo) ListStuckQueued(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	return nil, nil
}

func (r *hOrderRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	delete(r.orders, id)
	delete(r.byInvoice, snap.InvoiceID)
	return nil
}

var _ domain.OrderRepository = (*hOrderRepo)(nil)

type hQueue struct{}

func (q *hQueue) EnqueueGenerationTask(_ context.Context, _ uuid.UUID) error { return nil }
func (q *hQueue) EnqueueStatusCheckTask(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}

var _ domain.QueuePublisher = (*hQueue)(nil)

// hTxManager — заглушка Unit of Work для HTTP-тестов.
type hTxManager struct{}

func (hTxManager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

var _ usecase.TransactionManager = hTxManager{}
