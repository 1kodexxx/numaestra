package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_NotNil(t *testing.T) {
	if Handler() == nil {
		t.Fatal("Handler() должен возвращать ненулевой http.Handler")
	}
}

func TestHandler_Responds200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("ожидали 200, получили %d", rec.Code)
	}
}

func TestCounters_CanIncrement(t *testing.T) {
	OrdersCreated.Inc()
	OrdersCompleted.Inc()
	OrdersFailed.Inc()
	PaymentsReceived.Inc()
	SunoAPIErrors.Inc()
	LLMErrors.Inc()
	ActiveWorkerSlots.Inc()
	ActiveWorkerSlots.Dec()
}
