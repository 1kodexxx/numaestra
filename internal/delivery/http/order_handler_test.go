package apphttp

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
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
	pricing := usecase.NewStaticPricing(map[string]int64{"standard": 150000}, "standard")
	uc := usecase.NewOrderUseCase(repo, nil, &hQueue{}, nil, nil, nil, nil, usecase.NewNoopPromptUseCase(), pricing, hTxManager{}, discardLogger())
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

// --- CreateOrder ---

func TestHandler_CreateOrder_Success(t *testing.T) {
	_, router, _ := newTestHandler(t)

	body := `{"email":"user@example.com","brief":"Песня","plan":"standard"}`
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
	body := `{"email":"user@example.com","brief":"Песня","plan":"standard","amount_kopecks":1}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var resp OrderResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.AmountKopecks != 150000 {
		t.Errorf("клиентская сумma должна игнорироваться, ожидали 150000, получили %d", resp.AmountKopecks)
	}
}

func TestHandler_CreateOrder_UnknownPlan(t *testing.T) {
	_, router, _ := newTestHandler(t)
	body := `{"email":"user@example.com","brief":"Песня","plan":"vip-неизвестный"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 для неизвестного тарифа, получили %d", rec.Code)
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
	body := fmt.Sprintf(`{"email":"user@example.com","brief":%q,"plan":"standard"}`, longBrief)
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
		`{"brief":"Песня","plan":"standard"}`,        // нет контакта
		`{"email":"a@b.c","plan":"standard"}`,        // нет brief
		`{"phone":"+79990000000","plan":"standard"}`, // нет brief (только телефон)
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

// --- Webhook ---

func TestHandler_Webhook_Success(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф", "standard")

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
	order := mustCreate(t, h, "user@example.com", "", "Бриф", "standard")

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
	order := mustCreate(t, h, "user@example.com", "", "Бриф", "standard")

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
	order := mustCreate(t, h, "user@example.com", "", "Бриф", "standard")

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

func TestHandler_GetOrder_TokenForDifferentOrder(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф", "standard")

	// Токен валиден, но в URL чужой ID.
	req := httptest.NewRequest(http.MethodGet, "/"+uuid.NewString(), nil)
	req.Header.Set("X-Access-Token", order.AccessToken())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("ожидали 403 при несоответствии токена и ID, получили %d", rec.Code)
	}
}

func TestHandler_ListOrders_Success(t *testing.T) {
	h, router, _ := newTestHandler(t)
	order := mustCreate(t, h, "user@example.com", "", "Бриф", "standard")

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

// --- helpers ---

func mustCreate(t *testing.T, h *OrderHandler, email, phone, brief, plan string) *domain.Order {
	t.Helper()
	order, err := h.uc.CreateOrder(context.Background(), email, phone, brief, plan, "", nil)
	if err != nil {
		t.Fatalf("подготовка заказа: %v", err)
	}
	return order
}

// --- минимальные in-memory моки для конструирования use case ---

type hOrderRepo struct {
	mu        sync.Mutex
	orders    map[uuid.UUID]domain.OrderSnapshot
	byInvoice map[int64]uuid.UUID
	seq       int64
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
	id, ok := r.byInvoice[invoiceID]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return domain.RestoreOrder(r.orders[id]), nil
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

func (r *hOrderRepo) ListByCustomerEmail(_ context.Context, email string) ([]*domain.Order, error) {
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

func (r *hOrderRepo) ListByCustomerPhone(_ context.Context, phone string) ([]*domain.Order, error) {
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
