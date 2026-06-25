package apphttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/pkg/notify"
)

type feedbackNotifier struct {
	mu    sync.Mutex
	calls []notify.AdminFeedbackNotification
	err   error
}

func (n *feedbackNotifier) NotifyOrderComplete(_ context.Context, _ notify.OrderCompleteNotification) error {
	return nil
}

func (n *feedbackNotifier) NotifyAdminFeedback(_ context.Context, m notify.AdminFeedbackNotification) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls = append(n.calls, m)
	return n.err
}

func (n *feedbackNotifier) NotifyAccessLink(_ context.Context, _ notify.AccessLinkNotification) error {
	return nil
}

var _ notify.Notifier = (*feedbackNotifier)(nil)

func newTestAdminHandlerWithNotifier(t *testing.T) (*AdminHandler, *adminOrderRepo, *feedbackNotifier) {
	t.Helper()
	orders := newAdminOrderRepo()
	notifier := &feedbackNotifier{}
	uc := usecase.NewAdminUseCase(orders, newAdminAccRepo(), nil, &noopRefunder{}, nil, notifier, discardAdminLogger())
	return NewAdminHandler(uc, discardAdminLogger()), orders, notifier
}

func TestAdminHandler_SendOrderFeedback_Success(t *testing.T) {
	h, orders, notifier := newTestAdminHandlerWithNotifier(t)
	router := adminTestRouter(h)

	order, _ := domain.NewOrder(1, "client@example.com", "", "Бриф", "", "", domain.CurrentConsentDocVersion, 100)
	_ = orders.Create(context.Background(), order)

	body := `{"message":"Уточните, пожалуйста, дату праздника"}`
	r := httptest.NewRequest(http.MethodPost, "/admin/orders/"+order.ID().String()+"/feedback", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d (%s)", w.Code, w.Body.String())
	}

	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.calls) != 1 {
		t.Fatalf("ожидали 1 письмо, получили %d", len(notifier.calls))
	}
}

func TestAdminHandler_SendOrderFeedback_NotFound(t *testing.T) {
	h, _, _ := newTestAdminHandlerWithNotifier(t)
	router := adminTestRouter(h)

	body := `{"message":"текст"}`
	r := httptest.NewRequest(http.MethodPost, "/admin/orders/"+uuid.New().String()+"/feedback", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
}

func TestAdminHandler_SendOrderFeedback_InvalidUUID(t *testing.T) {
	h, _, _ := newTestAdminHandlerWithNotifier(t)
	router := adminTestRouter(h)

	body := `{"message":"текст"}`
	r := httptest.NewRequest(http.MethodPost, "/admin/orders/not-a-uuid/feedback", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
}

func TestAdminHandler_SendOrderFeedback_EmptyMessage(t *testing.T) {
	h, orders, _ := newTestAdminHandlerWithNotifier(t)
	router := adminTestRouter(h)

	order, _ := domain.NewOrder(1, "client@example.com", "", "Бриф", "", "", domain.CurrentConsentDocVersion, 100)
	_ = orders.Create(context.Background(), order)

	body := `{"message":""}`
	r := httptest.NewRequest(http.MethodPost, "/admin/orders/"+order.ID().String()+"/feedback", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 для пустого сообщения, получили %d", w.Code)
	}
}
