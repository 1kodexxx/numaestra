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
