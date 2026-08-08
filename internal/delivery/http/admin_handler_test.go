package apphttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/pkg/adminsession"
)

// adminTestRouter монтирует маршруты AdminHandler без auth-middleware.
func adminTestRouter(h *AdminHandler) http.Handler {
	r := chi.NewRouter()
	r.Mount("/admin", h.Routes())
	return r
}

func newTestAdminHandler(t *testing.T) (*AdminHandler, *adminOrderRepo, *adminAccRepo) {
	t.Helper()
	orders := newAdminOrderRepo()
	accounts := newAdminAccRepo()
	rk := &noopRefunder{}
	uc := usecase.NewAdminUseCase(orders, accounts, nil, rk, nil, nil, discardAdminLogger())
	return NewAdminHandler(uc, discardAdminLogger()), orders, accounts
}

func discardAdminLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- in-memory repos (isolated from usecase test package) ---

type adminOrderRepo struct {
	mu          sync.Mutex
	orders      map[uuid.UUID]domain.OrderSnapshot
	byInv       map[int64]uuid.UUID
	seq         int64
	getByIDErr  error
	countAllErr error
	listAllErr  error
	updateErr   error
}

func newAdminOrderRepo() *adminOrderRepo {
	return &adminOrderRepo{
		orders: make(map[uuid.UUID]domain.OrderSnapshot),
		byInv:  make(map[int64]uuid.UUID),
	}
}

func (r *adminOrderRepo) save(o *domain.Order) {
	s := o.Snapshot()
	r.orders[s.ID] = s
	r.byInv[s.InvoiceID] = s.ID
}

func (r *adminOrderRepo) Create(_ context.Context, o *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.save(o)
	return nil
}

func (r *adminOrderRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	s, ok := r.orders[id]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return domain.RestoreOrder(s), nil
}

func (r *adminOrderRepo) GetByInvoiceID(_ context.Context, inv int64) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byInv[inv]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return domain.RestoreOrder(r.orders[id]), nil
}

func (r *adminOrderRepo) GetByDemoInvoiceID(_ context.Context, inv int64) (*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.orders {
		if s.DemoInvoiceID != 0 && s.DemoInvoiceID == inv {
			return domain.RestoreOrder(s), nil
		}
	}
	return nil, domain.ErrOrderNotFound
}

func (r *adminOrderRepo) ApplyDemoPaymentSuccess(_ context.Context, o *domain.Order) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.orders[o.ID()]
	if !ok {
		return false, domain.ErrOrderNotFound
	}
	if cur.DemoPaymentStatus != domain.PaymentStatusPending {
		return false, nil
	}
	r.save(o)
	return true, nil
}

func (r *adminOrderRepo) SetAdminFeedback(_ context.Context, id uuid.UUID, feedback string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	s.AdminFeedback = feedback
	s.AdminFeedbackAt = &at
	r.orders[id] = s
	return nil
}

func (r *adminOrderRepo) Update(_ context.Context, o *domain.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
	r.save(o)
	return nil
}

func (r *adminOrderRepo) ApplyPaymentSuccess(_ context.Context, o *domain.Order) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.orders[o.ID()]
	if !ok {
		return false, domain.ErrOrderNotFound
	}
	if cur.PaymentStatus != domain.PaymentStatusPending {
		return false, nil
	}
	r.save(o)
	return true, nil
}

func (r *adminOrderRepo) ListByCustomerEmail(_ context.Context, _ string, _, _ int) ([]*domain.Order, error) {
	return nil, nil
}

func (r *adminOrderRepo) ListByCustomerPhone(_ context.Context, _ string, _, _ int) ([]*domain.Order, error) {
	return nil, nil
}

func (r *adminOrderRepo) NextInvoiceID(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	return r.seq, nil
}

func (r *adminOrderRepo) GetByAccessToken(_ context.Context, _ string) (*domain.Order, error) {
	return nil, domain.ErrOrderNotFound
}

func (r *adminOrderRepo) ListAll(_ context.Context, limit, offset int) ([]*domain.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listAllErr != nil {
		return nil, r.listAllErr
	}
	out := make([]*domain.Order, 0)
	i := 0
	for _, s := range r.orders {
		if i >= offset && len(out) < limit {
			out = append(out, domain.RestoreOrder(s))
		}
		i++
	}
	return out, nil
}

func (r *adminOrderRepo) CountAll(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.countAllErr != nil {
		return 0, r.countAllErr
	}
	return len(r.orders), nil
}

func (r *adminOrderRepo) ListStuckProcessing(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	return nil, nil
}

func (r *adminOrderRepo) ListStuckQueued(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	return nil, nil
}

func (r *adminOrderRepo) ListPendingPayment(_ context.Context, _, _ time.Time) ([]*domain.Order, error) {
	return nil, nil
}

func (r *adminOrderRepo) UpdateDemo(_ context.Context, _ *domain.Order) error { return nil }
func (r *adminOrderRepo) UpdatePromo(_ context.Context, _ *domain.Order) (bool, error) {
	return true, nil
}

func (r *adminOrderRepo) ListStuckDemo(_ context.Context, _ time.Time) ([]*domain.Order, error) {
	return nil, nil
}

func (r *adminOrderRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.orders[id]
	if !ok {
		return domain.ErrOrderNotFound
	}
	delete(r.orders, id)
	delete(r.byInv, snap.InvoiceID)
	return nil
}

var _ domain.OrderRepository = (*adminOrderRepo)(nil)

type adminAccRepo struct {
	mu        sync.Mutex
	accs      map[uuid.UUID]domain.SunoAccountSnapshot
	createErr error
	listErr   error
}

func newAdminAccRepo() *adminAccRepo {
	return &adminAccRepo{accs: make(map[uuid.UUID]domain.SunoAccountSnapshot)}
}

func (r *adminAccRepo) FetchAndLockAvailable(_ context.Context) (*domain.SunoAccount, error) {
	return nil, domain.ErrNoAvailableAccount
}

func (r *adminAccRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.SunoAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.accs[id]
	if !ok {
		return nil, domain.ErrAccountNotFound
	}
	return domain.RestoreSunoAccount(s), nil
}

func (r *adminAccRepo) Create(_ context.Context, a *domain.SunoAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	r.accs[a.ID()] = a.Snapshot()
	return nil
}

func (r *adminAccRepo) Update(_ context.Context, a *domain.SunoAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.accs[a.ID()] = a.Snapshot()
	return nil
}

func (r *adminAccRepo) ListByStatus(_ context.Context, _ domain.AccountStatus) ([]*domain.SunoAccount, error) {
	return nil, nil
}

func (r *adminAccRepo) List(_ context.Context) ([]*domain.SunoAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.listErr != nil {
		return nil, r.listErr
	}
	out := make([]*domain.SunoAccount, 0, len(r.accs))
	for _, s := range r.accs {
		out = append(out, domain.RestoreSunoAccount(s))
	}
	return out, nil
}

func (r *adminAccRepo) SetStatus(_ context.Context, id uuid.UUID, status domain.AccountStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.accs[id]
	if !ok {
		return domain.ErrAccountNotFound
	}
	s.Status = status
	r.accs[id] = s
	return nil
}

func (r *adminAccRepo) ResetAccount(_ context.Context, id uuid.UUID, releaseSlots bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.accs[id]
	if !ok {
		return domain.ErrAccountNotFound
	}
	s.Status = domain.AccountStatusActive
	s.FailureCount = 0
	s.CooldownUntil = nil
	if releaseSlots {
		s.ConcurrentTasks = 0
	}
	r.accs[id] = s
	return nil
}

var _ domain.AccountRepository = (*adminAccRepo)(nil)

type noopRefunder struct{ err error }

func (n *noopRefunder) Refund(_ context.Context, _ string, _ int64) error { return n.err }

type noopPaymentConfirmer struct {
	err error
}

func (n *noopPaymentConfirmer) AdminConfirmPayment(_ context.Context, _ uuid.UUID) error {
	return n.err
}

// --- тесты ---

func TestAdminHandler_ListAccounts_Empty(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/admin/accounts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", w.Code)
	}
	var resp []any
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp) != 0 {
		t.Errorf("ожидали пустой список, получили %d элементов", len(resp))
	}
}

func TestAdminHandler_AddAccount_Success(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	body := `{"email":"suno@test.com","session":"sess","max_concurrent":2}`
	r := httptest.NewRequest(http.MethodPost, "/admin/accounts", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d; тело: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["email"] != "suno@test.com" {
		t.Errorf("ожидали email=suno@test.com, получили %v", resp["email"])
	}
}

func TestAdminHandler_AddAccount_MissingFields(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	body := `{"email":"only@email.com"}`
	r := httptest.NewRequest(http.MethodPost, "/admin/accounts", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
}

func TestAdminHandler_SetAccountStatus_NotFound(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	body := `{"status":"banned"}`
	r := httptest.NewRequest(http.MethodPatch, "/admin/accounts/"+uuid.New().String(),
		strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
}

func TestAdminHandler_SetAccountStatus_InvalidUUID(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodPatch, "/admin/accounts/not-a-uuid",
		strings.NewReader(`{"status":"active"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
}

func TestAdminHandler_ListOrders_ReturnsPaginatedResult(t *testing.T) {
	h, orders, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	for i := int64(1); i <= 3; i++ {
		o, _ := domain.NewOrder(i, "u@a.com", "", "бриф", "", "", domain.CurrentConsentDocVersion, 100)
		_ = orders.Create(context.Background(), o)
	}

	r := httptest.NewRequest(http.MethodGet, "/admin/orders?per_page=2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["total"].(float64) != 3 {
		t.Errorf("ожидали total=3, получили %v", resp["total"])
	}
}

func TestAdminHandler_GetOrder_Found(t *testing.T) {
	h, orders, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	o, _ := domain.NewOrder(1, "x@y.com", "", "бриф", "", "", domain.CurrentConsentDocVersion, 200)
	_ = orders.Create(context.Background(), o)

	r := httptest.NewRequest(http.MethodGet, "/admin/orders/"+o.ID().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] != o.ID().String() {
		t.Errorf("ожидали id=%s, получили %v", o.ID(), resp["id"])
	}
}

func TestAdminHandler_GetOrder_NotFound(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/admin/orders/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
}

func TestAdminHandler_RefundOrder_Success(t *testing.T) {
	h, orders, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	o, _ := domain.NewOrder(1, "p@q.com", "", "бриф", "", "", domain.CurrentConsentDocVersion, 5000)
	_ = o.MarkPaid()
	_ = orders.Create(context.Background(), o)

	r := httptest.NewRequest(http.MethodPost, "/admin/orders/"+o.ID().String()+"/refund",
		bytes.NewReader(nil))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d; тело: %s", w.Code, w.Body.String())
	}
}

func TestAdminHandler_RefundOrder_NotPaid(t *testing.T) {
	h, orders, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	o, _ := domain.NewOrder(2, "p@q.com", "", "бриф", "", "", domain.CurrentConsentDocVersion, 5000)
	_ = orders.Create(context.Background(), o)

	r := httptest.NewRequest(http.MethodPost, "/admin/orders/"+o.ID().String()+"/refund",
		bytes.NewReader(nil))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 для неоплаченного заказа, получили %d", w.Code)
	}
}

func TestAdminHandler_ConfirmOrderPayment_Success(t *testing.T) {
	orders := newAdminOrderRepo()
	accounts := newAdminAccRepo()
	pc := &noopPaymentConfirmer{}
	uc := usecase.NewAdminUseCase(orders, accounts, nil, &noopRefunder{}, nil, nil, discardAdminLogger()).
		WithPaymentConfirmer(pc)
	h := NewAdminHandler(uc, discardAdminLogger())
	router := adminTestRouter(h)

	o, _ := domain.NewOrder(3, "p@q.com", "", "бриф", "", "", domain.CurrentConsentDocVersion, 5000)
	_ = orders.Create(context.Background(), o)

	r := httptest.NewRequest(http.MethodPost, "/admin/orders/"+o.ID().String()+"/confirm-payment", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d; тело: %s", w.Code, w.Body.String())
	}
}

func TestAdminHandler_ConfirmOrderPayment_NotFound(t *testing.T) {
	pc := &noopPaymentConfirmer{err: domain.ErrOrderNotFound}
	orders := newAdminOrderRepo()
	uc := usecase.NewAdminUseCase(orders, newAdminAccRepo(), nil, &noopRefunder{}, nil, nil, discardAdminLogger()).
		WithPaymentConfirmer(pc)
	h := NewAdminHandler(uc, discardAdminLogger())
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodPost, "/admin/orders/"+uuid.New().String()+"/confirm-payment", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
}

func TestAdminAuth_RequiresValidToken(t *testing.T) {
	const token = "secret"
	sessionSecret := []byte("01234567890123456789012345678901")
	h, _, _ := newTestAdminHandler(t)

	// Оборачиваем маршруты в AdminAuth (как в production).
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(AdminAuth(token, sessionSecret))
		r.Mount("/admin", h.Routes())
	})

	tests := []struct {
		name       string
		authHeader string
		wantCode   int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"wrong token", "Bearer wrong", http.StatusUnauthorized},
		{"correct token", "Bearer " + token, http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/accounts", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.wantCode {
				t.Errorf("ожидали %d, получили %d", tc.wantCode, w.Code)
			}
		})
	}
}

func TestAdminAuth_AcceptsValidSessionCookie(t *testing.T) {
	sessionSecret := []byte("01234567890123456789012345678901")
	h, _, _ := newTestAdminHandler(t)

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(AdminAuth("secret", sessionSecret))
		r.Mount("/admin", h.Routes())
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/accounts", nil)
	req.AddCookie(&http.Cookie{Name: AdminSessionCookieName, Value: adminsession.Issue(sessionSecret, "owner", time.Hour)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200 с валидной cookie-сессией, получили %d (%s)", w.Code, w.Body.String())
	}
}

func TestAdminAuth_RejectsExpiredSessionCookie(t *testing.T) {
	sessionSecret := []byte("01234567890123456789012345678901")
	h, _, _ := newTestAdminHandler(t)

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(AdminAuth("secret", sessionSecret))
		r.Mount("/admin", h.Routes())
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/accounts", nil)
	req.AddCookie(&http.Cookie{Name: AdminSessionCookieName, Value: adminsession.Issue(sessionSecret, "owner", -time.Hour)})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("ожидали 401 для истёкшей cookie-сессии, получили %d", w.Code)
	}
}

// --- error paths ---

func TestAdminHandler_ListAccounts_InternalError(t *testing.T) {
	h, _, accounts := newTestAdminHandler(t)
	accounts.listErr = errors.New("db down")
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/admin/accounts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("ожидали 500, получили %d", w.Code)
	}
}

func TestAdminHandler_AddAccount_InvalidJSON(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodPost, "/admin/accounts", strings.NewReader("{bad json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
}

func TestAdminHandler_AddAccount_InternalError(t *testing.T) {
	h, _, accounts := newTestAdminHandler(t)
	accounts.createErr = errors.New("db down")
	router := adminTestRouter(h)

	body := `{"email":"x@y.com","session":"sess","max_concurrent":1}`
	r := httptest.NewRequest(http.MethodPost, "/admin/accounts", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("ожидали 500, получили %d", w.Code)
	}
}

func TestAdminHandler_SetAccountStatus_BadJSON(t *testing.T) {
	h, _, accounts := newTestAdminHandler(t)
	router := adminTestRouter(h)
	acc, _ := domain.NewSunoAccount("x@y.com", "s", 5)
	_ = accounts.Create(context.Background(), acc)

	r := httptest.NewRequest(http.MethodPatch, "/admin/accounts/"+acc.ID().String(),
		strings.NewReader("{bad json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
}

func TestAdminHandler_SetAccountStatus_InvalidStatus(t *testing.T) {
	h, _, accounts := newTestAdminHandler(t)
	router := adminTestRouter(h)
	acc, _ := domain.NewSunoAccount("x@y.com", "s", 5)
	_ = accounts.Create(context.Background(), acc)

	r := httptest.NewRequest(http.MethodPatch, "/admin/accounts/"+acc.ID().String(),
		strings.NewReader(`{"status":"unknown-status"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 для недопустимого статуса, получили %d", w.Code)
	}
}

func TestAdminHandler_ListOrders_InternalError(t *testing.T) {
	h, orders, _ := newTestAdminHandler(t)
	orders.countAllErr = errors.New("db down")
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/admin/orders", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("ожидали 500, получили %d", w.Code)
	}
}

func TestAdminHandler_GetOrder_InvalidUUID(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/admin/orders/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
}

func TestAdminHandler_GetOrder_InternalError(t *testing.T) {
	h, orders, _ := newTestAdminHandler(t)
	orders.getByIDErr = errors.New("db down")
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/admin/orders/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("ожидали 500, получили %d", w.Code)
	}
}

func TestAdminHandler_RefundOrder_InvalidUUID(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodPost, "/admin/orders/bad-uuid/refund", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
}

func TestAdminHandler_RefundOrder_NotFoundViaRouter(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodPost, "/admin/orders/"+uuid.New().String()+"/refund", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404 для несуществующего заказа, получили %d", w.Code)
	}
}

func TestAdminHandler_RefundOrder_UpdateError(t *testing.T) {
	h, orders, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	o, _ := domain.NewOrder(1, "a@b.com", "", "бриф", "", "", domain.CurrentConsentDocVersion, 5000)
	_ = o.MarkPaid()
	_ = orders.Create(context.Background(), o)
	orders.updateErr = errors.New("db down")

	r := httptest.NewRequest(http.MethodPost, "/admin/orders/"+o.ID().String()+"/refund", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 при ошибке возврата, получили %d", w.Code)
	}
}

// Проверяем, что пустой ADMIN_TOKEN блокирует все запросы.
func TestAdminAuth_EmptyTokenBlocksAll(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.Use(AdminAuth("", []byte("01234567890123456789012345678901"))) // пустой токен
		r.Mount("/admin", h.Routes())
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/accounts", nil)
	req.Header.Set("Authorization", "Bearer anything")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("пустой токен должен блокировать все запросы, получили %d", w.Code)
	}
}

func TestAdminHandler_DeleteOrder_Success(t *testing.T) {
	h, orders, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	o, _ := domain.NewOrder(1, "del@e.c", "", "бриф", "", "", domain.CurrentConsentDocVersion, 100)
	_ = orders.Create(context.Background(), o)

	r := httptest.NewRequest(http.MethodDelete, "/admin/orders/"+o.ID().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d (%s)", w.Code, w.Body.String())
	}
	if _, err := orders.GetByID(context.Background(), o.ID()); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Error("заказ должен быть удалён")
	}
}

func TestAdminHandler_DeleteOrder_NotFound(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodDelete, "/admin/orders/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
}

func TestAdminHandler_DeleteOrder_InvalidUUID(t *testing.T) {
	h, _, _ := newTestAdminHandler(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodDelete, "/admin/orders/not-a-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
}

func TestAdminHandler_DeleteOrder_WithStorage(t *testing.T) {
	orders := newAdminOrderRepo()
	accounts := newAdminAccRepo()
	storage := &deleteTrackingStorage{}
	uc := usecase.NewAdminUseCase(orders, accounts, nil, &noopRefunder{}, nil, nil, discardAdminLogger()).
		WithStorage(storage)
	h := NewAdminHandler(uc, discardAdminLogger())
	router := adminTestRouter(h)

	o, _ := domain.NewOrder(1, "stor@e.c", "", "бриф", "", "", domain.CurrentConsentDocVersion, 100)
	_ = orders.Create(context.Background(), o)

	r := httptest.NewRequest(http.MethodDelete, "/admin/orders/"+o.ID().String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d (%s)", w.Code, w.Body.String())
	}
	if !storage.called {
		t.Error("ожидали вызов DeleteOrderTracks перед удалением из БД")
	}
}

type deleteTrackingStorage struct {
	called bool
}

func (s *deleteTrackingStorage) UploadFromURL(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (s *deleteTrackingStorage) Upload(context.Context, string, string, []byte) (string, error) {
	return "", nil
}

func (s *deleteTrackingStorage) DeleteOrderTracks(_ context.Context, _ uuid.UUID) error {
	s.called = true
	return nil
}

func (s *deleteTrackingStorage) DeleteByURL(_ context.Context, _ string) error {
	return nil
}

func (s *deleteTrackingStorage) ResolvePlayURL(_ context.Context, storedURL string, _ time.Duration) (string, error) {
	return storedURL, nil
}
