package apphttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
)

func TestPublicHandler_GetConfig(t *testing.T) {
	h := NewPublicHandler(99000, 200000, false)
	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	w := httptest.NewRecorder()
	h.GetConfig(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var body publicConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.PriceKopecks != 99000 || body.PriceLabel != "990 ₽" {
		t.Errorf("price: %+v", body)
	}
	if body.OldPriceLabel != "2000 ₽" {
		t.Errorf("old price: %+v", body)
	}
	if body.DemoEnabled {
		t.Error("demo_enabled должен быть false, если демо выключено")
	}
	if body.ConsentDocVersion != domain.CurrentConsentDocVersion {
		t.Errorf("consent version: %q", body.ConsentDocVersion)
	}
}

func TestPublicHandler_GetConfig_DemoEnabled(t *testing.T) {
	h := NewPublicHandler(99000, 0, true)
	w := httptest.NewRecorder()
	h.GetConfig(w, httptest.NewRequest(http.MethodGet, "/config", nil))

	var body publicConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.DemoEnabled {
		t.Error("demo_enabled должен быть true, если демо включено")
	}
}

// Старая цена не отдаётся, если она не выше текущей (0 = выключено).
func TestPublicHandler_GetConfig_NoOldPrice(t *testing.T) {
	for _, oldPrice := range []int64{0, 99000, 50000} {
		h := NewPublicHandler(99000, oldPrice, false)
		w := httptest.NewRecorder()
		h.GetConfig(w, httptest.NewRequest(http.MethodGet, "/config", nil))

		var body publicConfigResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.OldPriceLabel != "" {
			t.Errorf("old_price_kopecks=%d: старая цена не должна отдаваться, получили %q", oldPrice, body.OldPriceLabel)
		}
	}
}

// Витрина получает обе цены воронки: сколько стоит демо и сколько останется доплатить.
func TestPublicHandler_GetConfig_ЦеныПлатногоДемо(t *testing.T) {
	h := NewPublicHandler(99000, 200000, true).WithDemoPrice(5000)
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/config", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	var body publicConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.DemoPriceKopecks != 5000 || body.DemoPriceLabel != "50 ₽" {
		t.Errorf("цена демо: kopecks=%d label=%q, ожидалось 5000 / «50 ₽»", body.DemoPriceKopecks, body.DemoPriceLabel)
	}
	if body.RemainingPriceLabel != "940 ₽" {
		t.Errorf("остаток = %q, ожидалось «940 ₽»", body.RemainingPriceLabel)
	}
}

// Цены демо не отдаются, когда второй счёт не выставляется: демо выключено,
// цена нулевая или не дешевле заказа (иначе к доплате остался бы 0).
func TestPublicHandler_GetConfig_БезЦеныДемо(t *testing.T) {
	cases := []struct {
		name        string
		demoEnabled bool
		demoPrice   int64
	}{
		{"демо выключено", false, 5000},
		{"нулевая цена демо", true, 0},
		{"демо не дешевле заказа", true, 99000},
		{"демо дороже заказа", true, 150000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewPublicHandler(99000, 0, tc.demoEnabled).WithDemoPrice(tc.demoPrice)
			w := httptest.NewRecorder()
			h.GetConfig(w, httptest.NewRequest(http.MethodGet, "/config", nil))

			var body publicConfigResponse
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.DemoPriceKopecks != 0 || body.DemoPriceLabel != "" || body.RemainingPriceLabel != "" {
				t.Errorf("цены демо не должны отдаваться: %+v", body)
			}
		})
	}
}

// Копейки в метке цены не теряются (990,50 ₽), иначе витрина врёт о сумме.
func TestPublicHandler_GetConfig_ЦенаСКопейками(t *testing.T) {
	h := NewPublicHandler(99050, 0, false)
	w := httptest.NewRecorder()
	h.GetConfig(w, httptest.NewRequest(http.MethodGet, "/config", nil))

	var body publicConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.PriceLabel != "990,50 ₽" {
		t.Errorf("price_label = %q, ожидалось «990,50 ₽»", body.PriceLabel)
	}
}
