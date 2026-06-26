package apphttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/usecase"
)

func newAdminPromoRouter(t *testing.T) (chi.Router, *usecase.PromoUseCase) {
	t.Helper()
	repo := newHPromoRepoTyped()
	uc := usecase.NewPromoUseCase(repo, discardLogger())
	h := NewAdminPromoHandler(uc, discardLogger())
	r := chi.NewRouter()
	r.Mount("/admin/promo", h.Routes())
	return r, uc
}

func TestAdminPromoHandler_List_Empty(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/promo/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["total"].(float64) != 0 {
		t.Error("ожидали total=0")
	}
}

func TestAdminPromoHandler_Create_Success(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	body := `{"code":"ADMIN10","discount_type":"percent","discount_value":10,"description":"тест"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/promo/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d: %s", rec.Code, rec.Body.String())
	}
	var resp promoResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("декодирование ответа: %v", err)
	}
	if resp.Code != "ADMIN10" {
		t.Errorf("неверный code: %q", resp.Code)
	}
	if resp.Description != "тест" {
		t.Errorf("неверное description: %q", resp.Description)
	}
	if resp.IsActive != true {
		t.Error("новый промокод должен быть активен")
	}
}

func TestAdminPromoHandler_Create_MissingFields(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	cases := []struct {
		body string
		desc string
	}{
		{`{"discount_type":"percent","discount_value":10}`, "нет code"},
		{`{"code":"X","discount_value":10}`, "нет discount_type"},
		{`{"code":"X","discount_type":"percent","discount_value":0}`, "discount_value=0"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/admin/promo/", bytes.NewBufferString(c.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("[%s] ожидали 400, получили %d", c.desc, rec.Code)
		}
	}
}

func TestAdminPromoHandler_Create_InvalidJSON(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/promo/", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", rec.Code)
	}
}

func TestAdminPromoHandler_Create_WithDates(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	body := `{"code":"DATED","discount_type":"percent","discount_value":5,"valid_from":"2025-01-01T00:00:00Z","valid_until":"2030-12-31T23:59:59Z"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/promo/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d: %s", rec.Code, rec.Body.String())
	}
	var resp promoResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ValidFrom == nil {
		t.Error("ожидали valid_from в ответе")
	}
	if resp.ValidUntil == nil {
		t.Error("ожидали valid_until в ответе")
	}
}

func TestAdminPromoHandler_Get_Found(t *testing.T) {
	r, uc := newAdminPromoRouter(t)
	p := mustCreatePromo(t, uc, "GETME", "fixed_rub", 300)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/promo/%s", p.ID()), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", rec.Code, rec.Body.String())
	}
	var resp promoResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Code != "GETME" {
		t.Errorf("неверный code: %q", resp.Code)
	}
	if resp.DiscountType != "fixed_rub" {
		t.Errorf("неверный discount_type: %q", resp.DiscountType)
	}
}

func TestAdminPromoHandler_Get_NotFound(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/promo/%s", uuid.New()), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", rec.Code)
	}
}

func TestAdminPromoHandler_Get_InvalidID(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/promo/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", rec.Code)
	}
}

func TestAdminPromoHandler_Update_Success(t *testing.T) {
	r, uc := newAdminPromoRouter(t)
	p := mustCreatePromo(t, uc, "UPME", "percent", 5)

	body := `{"description":"обновлено","is_active":true,"max_uses":100}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/promo/%s", p.ID()), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", rec.Code, rec.Body.String())
	}
	var resp promoResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Description != "обновлено" {
		t.Errorf("description не обновился: %q", resp.Description)
	}
	if resp.MaxUses == nil || *resp.MaxUses != 100 {
		t.Errorf("max_uses не обновился: %v", resp.MaxUses)
	}
}

func TestAdminPromoHandler_Update_NotFound(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	body := `{"is_active":false}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/promo/%s", uuid.New()), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", rec.Code)
	}
}

func TestAdminPromoHandler_Update_InvalidID(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	body := `{"is_active":true}`
	req := httptest.NewRequest(http.MethodPut, "/admin/promo/bad-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", rec.Code)
	}
}

func TestAdminPromoHandler_Update_InvalidJSON(t *testing.T) {
	r, uc := newAdminPromoRouter(t)
	p := mustCreatePromo(t, uc, "BADJSON", "percent", 5)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/promo/%s", p.ID()), bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", rec.Code)
	}
}

func TestAdminPromoHandler_Delete_Success(t *testing.T) {
	r, uc := newAdminPromoRouter(t)
	p := mustCreatePromo(t, uc, "DELME", "percent", 5)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/promo/%s", p.ID()), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", rec.Code, rec.Body.String())
	}

	// Проверяем, что теперь не находится
	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/promo/%s", p.ID()), nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotFound {
		t.Errorf("после удаления ожидали 404, получили %d", rec2.Code)
	}
}

func TestAdminPromoHandler_Delete_NotFound(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/promo/%s", uuid.New()), nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", rec.Code)
	}
}

func TestAdminPromoHandler_Delete_InvalidID(t *testing.T) {
	r, _ := newAdminPromoRouter(t)

	req := httptest.NewRequest(http.MethodDelete, "/admin/promo/bad-id", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", rec.Code)
	}
}

func TestAdminPromoHandler_List_AfterCreate(t *testing.T) {
	r, uc := newAdminPromoRouter(t)
	mustCreatePromo(t, uc, "P1", "percent", 10)
	mustCreatePromo(t, uc, "P2", "fixed_rub", 200)

	req := httptest.NewRequest(http.MethodGet, "/admin/promo/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", rec.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp["total"].(float64) != 2 {
		t.Errorf("ожидали total=2, получили %v", resp["total"])
	}
}

func TestAdminPromoHandler_Update_WithDates(t *testing.T) {
	r, uc := newAdminPromoRouter(t)
	p := mustCreatePromo(t, uc, "DATES2", "percent", 7)

	body := `{"is_active":true,"valid_from":"2025-06-01T00:00:00Z","valid_until":"2030-01-01T00:00:00Z"}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/promo/%s", p.ID()), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d: %s", rec.Code, rec.Body.String())
	}
	var resp promoResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ValidFrom == nil {
		t.Error("ожидали valid_from после обновления")
	}
}

// promoToResponse покрывается через Create/Get, но проверим explicitly через CurrentUses
func TestPromoToResponse_CurrentUses(t *testing.T) {
	_, uc := newAdminPromoRouter(t)
	p := mustCreatePromo(t, uc, "USES", "percent", 5)

	resp := promoToResponse(p)
	if resp.CurrentUses != 0 {
		t.Errorf("ожидали current_uses=0, получили %d", resp.CurrentUses)
	}
	if resp.CreatedAt == "" {
		t.Error("created_at должен быть установлен")
	}
	if resp.ID == "" {
		t.Error("id должен быть установлен")
	}
}
