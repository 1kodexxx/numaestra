package apphttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
)

func TestPublicHandler_GetConfig(t *testing.T) {
	h := NewPublicHandler(200000)
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
	if body.PriceKopecks != 200000 || body.PriceLabel != "2000 ₽" {
		t.Errorf("price: %+v", body)
	}
	if body.ConsentDocVersion != domain.CurrentConsentDocVersion {
		t.Errorf("consent version: %q", body.ConsentDocVersion)
	}
}
