package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
)

func newAdminUCWithNotifier(t *testing.T) (*AdminUseCase, *inMemOrderRepo, *mockNotifier) {
	t.Helper()
	orders := newInMemOrderRepo()
	notifier := &mockNotifier{}
	uc := NewAdminUseCase(orders, newInMemAccountRepo(), nil, &mockRefunder{}, nil, notifier, testLogger())
	return uc, orders, notifier
}

func TestAdminUseCase_SendOrderFeedback_Success(t *testing.T) {
	uc, orders, notifier := newAdminUCWithNotifier(t)
	order, err := domain.NewOrder(1, "client@example.com", "", "Бриф", "", "", 100)
	if err != nil {
		t.Fatalf("создание заказа: %v", err)
	}
	if err := orders.Create(context.Background(), order); err != nil {
		t.Fatalf("сохранение заказа: %v", err)
	}

	if err := uc.SendOrderFeedback(context.Background(), order.ID(), "Уточните, пожалуйста, имя именинника"); err != nil {
		t.Fatalf("SendOrderFeedback упал: %v", err)
	}

	got, err := orders.GetByID(context.Background(), order.ID())
	if err != nil {
		t.Fatalf("получение заказа: %v", err)
	}
	if got.AdminFeedback() != "Уточните, пожалуйста, имя именинника" {
		t.Errorf("обратная связь не сохранена, получили %q", got.AdminFeedback())
	}
	if got.AdminFeedbackAt() == nil {
		t.Error("ожидали проставленную метку времени обратной связи")
	}

	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.feedbackCalls) != 1 {
		t.Fatalf("ожидали 1 отправленное письмо, получили %d", len(notifier.feedbackCalls))
	}
	if notifier.feedbackCalls[0].Email != "client@example.com" {
		t.Errorf("письмо должно уйти на email клиента, получили %q", notifier.feedbackCalls[0].Email)
	}
}

func TestAdminUseCase_SendOrderFeedback_OrderNotFound(t *testing.T) {
	uc, _, _ := newAdminUCWithNotifier(t)
	err := uc.SendOrderFeedback(context.Background(), uuid.New(), "сообщение")
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("ожидали ErrOrderNotFound, получили %v", err)
	}
}

func TestAdminUseCase_SendOrderFeedback_EmptyMessage(t *testing.T) {
	uc, orders, _ := newAdminUCWithNotifier(t)
	order, _ := domain.NewOrder(1, "client@example.com", "", "Бриф", "", "", 100)
	_ = orders.Create(context.Background(), order)

	if err := uc.SendOrderFeedback(context.Background(), order.ID(), "   "); err == nil {
		t.Error("ожидали ошибку валидации для пустого сообщения")
	}
}

func TestAdminUseCase_SendOrderFeedback_NotifierError(t *testing.T) {
	uc, orders, notifier := newAdminUCWithNotifier(t)
	order, _ := domain.NewOrder(1, "client@example.com", "", "Бриф", "", "", 100)
	_ = orders.Create(context.Background(), order)
	notifier.feedbackErr = errors.New("smtp down")

	err := uc.SendOrderFeedback(context.Background(), order.ID(), "сообщение")
	if err == nil {
		t.Fatal("ожидали ошибку при сбое отправки письма")
	}

	// Несмотря на сбой письма, фидбек должен быть сохранён в БД для истории.
	got, _ := orders.GetByID(context.Background(), order.ID())
	if got.AdminFeedback() != "сообщение" {
		t.Error("обратная связь должна сохраняться в БД даже при сбое отправки письма")
	}
}
