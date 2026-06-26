package apphttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

// hPromoRepoTyped — in-memory реализация domain.PromoCodeRepository для handler-тестов.
type hPromoRepoTyped struct {
	byCode map[string]*domain.PromoCode
	byID   map[uuid.UUID]*domain.PromoCode
}

func newHPromoRepoTyped() *hPromoRepoTyped {
	return &hPromoRepoTyped{
		byCode: make(map[string]*domain.PromoCode),
		byID:   make(map[uuid.UUID]*domain.PromoCode),
	}
}

func (r *hPromoRepoTyped) GetByCode(_ context.Context, code string) (*domain.PromoCode, error) {
	p, ok := r.byCode[code]
	if !ok {
		return nil, domain.ErrPromoCodeNotFound
	}
	return p, nil
}

func (r *hPromoRepoTyped) GetByID(_ context.Context, id uuid.UUID) (*domain.PromoCode, error) {
	p, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrPromoCodeNotFound
	}
	return p, nil
}

func (r *hPromoRepoTyped) Create(_ context.Context, p *domain.PromoCode) error {
	r.byCode[p.Code()] = p
	r.byID[p.ID()] = p
	return nil
}

func (r *hPromoRepoTyped) Update(_ context.Context, p *domain.PromoCode) error {
	r.byCode[p.Code()] = p
	r.byID[p.ID()] = p
	return nil
}

func (r *hPromoRepoTyped) Delete(_ context.Context, id uuid.UUID) error {
	p, ok := r.byID[id]
	if !ok {
		return domain.ErrPromoCodeNotFound
	}
	delete(r.byCode, p.Code())
	delete(r.byID, id)
	return nil
}

func (r *hPromoRepoTyped) List(_ context.Context, _, _ int) ([]*domain.PromoCode, error) {
	return nil, nil
}

func (r *hPromoRepoTyped) Count(_ context.Context) (int, error) { return len(r.byID), nil }

func (r *hPromoRepoTyped) IncrementUses(_ context.Context, _ uuid.UUID) (bool, error) {
	return true, nil
}

func newPromoHandlerRouter(t *testing.T) (chi.Router, *usecase.PromoUseCase) {
	t.Helper()
	repo := newHPromoRepoTyped()
	uc := usecase.NewPromoUseCase(repo, discardLogger())
	h := NewPromoHandler(uc, discardLogger())
	r := chi.NewRouter()
	r.Mount("/promo", h.Routes())
	return r, uc
}

func mustCreatePromo(t *testing.T, uc *usecase.PromoUseCase, code string, dtype string, value int) *domain.PromoCode {
	t.Helper()
	p, err := uc.CreatePromoCode(context.Background(), usecase.CreatePromoRequest{
		Code: code, DiscountType: dtype, DiscountValue: value,
	})
	if err != nil {
		t.Fatalf("mustCreatePromo %q: %v", code, err)
	}
	return p
}

func TestPromoHandler_ValidatePromo_Found(t *testing.T) {
	r, uc := newPromoHandlerRouter(t)
	mustCreatePromo(t, uc, "SAVE10", "percent", 10)

	req := httptest.NewRequest(http.MethodGet, "/promo/SAVE10", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", rec.Code, rec.Body.String())
	}
	var resp PromoValidateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("декодирование ответа: %v", err)
	}
	if resp.Code != "SAVE10" {
		t.Errorf("неверный code: %q", resp.Code)
	}
	if resp.DiscountType != "percent" {
		t.Errorf("неверный discount_type: %q", resp.DiscountType)
	}
	if resp.DiscountValue != 10 {
		t.Errorf("неверный discount_value: %d", resp.DiscountValue)
	}
}

func TestPromoHandler_ValidatePromo_WithAmount(t *testing.T) {
	r, uc := newPromoHandlerRouter(t)
	mustCreatePromo(t, uc, "FLAT500", "fixed_rub", 500)

	req := httptest.NewRequest(http.MethodGet, "/promo/FLAT500?amount_kopecks=150000", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", rec.Code, rec.Body.String())
	}
	var resp PromoValidateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("декодирование ответа: %v", err)
	}
	if resp.DiscountKopecks != 50000 {
		t.Errorf("ожидали discount_kopecks=50000, получили %d", resp.DiscountKopecks)
	}
}

func TestPromoHandler_ValidatePromo_NotFound(t *testing.T) {
	r, _ := newPromoHandlerRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/promo/MISSING", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", rec.Code)
	}
}

func TestPromoHandler_ValidatePromo_Inactive(t *testing.T) {
	r, uc := newPromoHandlerRouter(t)
	p := mustCreatePromo(t, uc, "INACTIVE", "percent", 5)
	_, _ = uc.UpdatePromoCode(context.Background(), p.ID(), usecase.UpdatePromoRequest{IsActive: false})

	req := httptest.NewRequest(http.MethodGet, "/promo/INACTIVE", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404 для неактивного кода, получили %d", rec.Code)
	}
}

func TestPromoHandler_ValidatePromo_InvalidAmount(t *testing.T) {
	r, uc := newPromoHandlerRouter(t)
	mustCreatePromo(t, uc, "CODE5", "percent", 5)

	req := httptest.NewRequest(http.MethodGet, "/promo/CODE5?amount_kopecks=abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", rec.Code)
	}
	var resp PromoValidateResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.DiscountKopecks != 0 {
		t.Errorf("невалидный amount_kopecks должен давать discount_kopecks=0, получили %d", resp.DiscountKopecks)
	}
}

func TestPromoHandler_ValidatePromo_LowercaseCode(t *testing.T) {
	r, uc := newPromoHandlerRouter(t)
	mustCreatePromo(t, uc, "UPPER", "percent", 15)

	// Запрос с lowercase — хендлер нормализует через strings.ToUpper
	req := httptest.NewRequest(http.MethodGet, "/promo/upper", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200 для lowercase кода, получили %d: %s", rec.Code, rec.Body.String())
	}
}
