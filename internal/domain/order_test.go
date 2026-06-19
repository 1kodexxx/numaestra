package domain

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

// --- NewOrder ---

func TestNewOrder_Valid(t *testing.T) {
	o, err := NewOrder(1, "user@example.com", "+79991234567", "Весёлая песня", "", "", 150000)
	if err != nil {
		t.Fatalf("ожидали успех, получили ошибку: %v", err)
	}
	if o.ID() == (uuid.UUID{}) {
		t.Error("ID не должен быть нулевым")
	}
	if o.PaymentStatus() != PaymentStatusPending {
		t.Errorf("ожидали статус оплаты %q, получили %q", PaymentStatusPending, o.PaymentStatus())
	}
	if o.GenerationStatus() != GenerationStatusNew {
		t.Errorf("ожидали статус генерации %q, получили %q", GenerationStatusNew, o.GenerationStatus())
	}
}

func TestNewOrder_GeneratesUniqueAccessTokens(t *testing.T) {
	o1, _ := NewOrder(1, "a@example.com", "", "Бриф 1", "", "", 100)
	o2, _ := NewOrder(2, "b@example.com", "", "Бриф 2", "", "", 100)

	if o1.AccessToken() == "" {
		t.Error("токен не должен быть пустым")
	}
	if o1.AccessToken() == o2.AccessToken() {
		t.Error("токены двух разных заказов не должны совпадать")
	}
	if len(o1.AccessToken()) != 64 {
		t.Errorf("ожидали длину токена 64 символа, получили %d", len(o1.AccessToken()))
	}
}

func TestNewOrder_RequiresContact(t *testing.T) {
	_, err := NewOrder(1, "", "", "Бриф", "", "", 100)
	if err == nil {
		t.Error("ожидали ошибку при отсутствии email и телефона")
	}
}

func TestNewOrder_RequiresBrief(t *testing.T) {
	_, err := NewOrder(1, "user@example.com", "", "", "", "", 100)
	if err == nil {
		t.Error("ожидали ошибку при пустом брифе")
	}
}

func TestNewOrder_RejectsTooLongBrief(t *testing.T) {
	longBrief := strings.Repeat("я", MaxBriefLength+1)
	_, err := NewOrder(1, "user@example.com", "", longBrief, "", "", 100)
	if err == nil {
		t.Fatal("ожидали ошибку при слишком длинном брифе")
	}
	if err != ErrBriefTooLong {
		t.Errorf("ожидали ErrBriefTooLong, получили %v", err)
	}

	// Бриф ровно на границе длины должен приниматься.
	maxBrief := strings.Repeat("я", MaxBriefLength)
	if _, err := NewOrder(1, "user@example.com", "", maxBrief, "", "", 100); err != nil {
		t.Errorf("бриф длиной ровно MaxBriefLength должен приниматься, получили %v", err)
	}
}

func TestNewOrder_RequiresPositiveAmount(t *testing.T) {
	_, err := NewOrder(1, "user@example.com", "", "Бриф", "", "", 0)
	if err == nil {
		t.Error("ожидали ошибку при нулевой сумме")
	}
	_, err = NewOrder(1, "user@example.com", "", "Бриф", "", "", -100)
	if err == nil {
		t.Error("ожидали ошибку при отрицательной сумме")
	}
}

func TestNewOrder_OnlyPhone(t *testing.T) {
	_, err := NewOrder(1, "", "+79991234567", "Бриф", "", "", 100)
	if err != nil {
		t.Errorf("телефон без email должен быть допустим: %v", err)
	}
}

// --- Стейт-машина оплаты ---

func TestOrder_MarkPaid(t *testing.T) {
	o := newTestOrder(t)

	if err := o.MarkPaid(); err != nil {
		t.Fatalf("MarkPaid упал: %v", err)
	}
	if o.PaymentStatus() != PaymentStatusPaid {
		t.Errorf("ожидали %q, получили %q", PaymentStatusPaid, o.PaymentStatus())
	}
}

func TestOrder_MarkPaid_Idempotency(t *testing.T) {
	o := newTestOrder(t)
	_ = o.MarkPaid()

	err := o.MarkPaid()
	if err != ErrOrderAlreadyPaid {
		t.Errorf("повторный MarkPaid должен возвращать ErrOrderAlreadyPaid, получили: %v", err)
	}
}

func TestOrder_MarkPaymentFailed(t *testing.T) {
	o := newTestOrder(t)
	if err := o.MarkPaymentFailed(); err != nil {
		t.Fatalf("MarkPaymentFailed упал: %v", err)
	}
	if o.PaymentStatus() != PaymentStatusFailed {
		t.Errorf("ожидали %q, получили %q", PaymentStatusFailed, o.PaymentStatus())
	}
}

// --- Стейт-машина генерации ---

func TestOrder_GenerationLifecycle_HappyPath(t *testing.T) {
	o := newPaidOrder(t)
	accountID := uuid.New()

	// Pending → Queued
	if err := o.Enqueue(); err != nil {
		t.Fatalf("Enqueue упал: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusQueued {
		t.Errorf("ожидали Queued, получили %q", o.GenerationStatus())
	}

	// Queued → Processing
	if err := o.StartProcessing(accountID); err != nil {
		t.Fatalf("StartProcessing упал: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusProcessing {
		t.Errorf("ожидали Processing, получили %q", o.GenerationStatus())
	}

	// Processing → Completed
	tracks := []Track{{ID: uuid.New(), Index: 1, AudioURL: "https://s3/track1.mp3"}}
	if err := o.Complete(tracks); err != nil {
		t.Fatalf("Complete упал: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusCompleted {
		t.Errorf("ожидали Completed, получили %q", o.GenerationStatus())
	}
	if len(o.Tracks()) != 1 {
		t.Errorf("ожидали 1 трек, получили %d", len(o.Tracks()))
	}
}

func TestOrder_Enqueue_RequiresPaid(t *testing.T) {
	o := newTestOrder(t) // статус Pending
	err := o.Enqueue()
	if err != ErrOrderNotPaid {
		t.Errorf("Enqueue без оплаты должен вернуть ErrOrderNotPaid, получили: %v", err)
	}
}

func TestOrder_Complete_RequiresTracks(t *testing.T) {
	o := newProcessingOrder(t)
	err := o.Complete(nil)
	if err == nil {
		t.Error("Complete с пустым списком треков должен вернуть ошибку")
	}
}

func TestOrder_RequeueForRetry(t *testing.T) {
	o := newProcessingOrder(t)
	if err := o.RequeueForRetry(); err != nil {
		t.Fatalf("RequeueForRetry упал: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusQueued {
		t.Errorf("ожидали Queued после retry, получили %q", o.GenerationStatus())
	}
	if o.AssignedAccountID() != nil {
		t.Error("AssignedAccountID должен быть nil после retry")
	}
}

func TestOrder_Fail(t *testing.T) {
	o := newProcessingOrder(t)
	if err := o.Fail("превышен лимит ретраев"); err != nil {
		t.Fatalf("Fail упал: %v", err)
	}
	if o.GenerationStatus() != GenerationStatusFailed {
		t.Errorf("ожидали Failed, получили %q", o.GenerationStatus())
	}
	if !strings.Contains(o.FailureReason(), "лимит") {
		t.Errorf("FailureReason должен содержать причину, получили %q", o.FailureReason())
	}
}

// --- Snapshot / Restore ---

func TestOrder_SnapshotRestore_PreservesToken(t *testing.T) {
	original, _ := NewOrder(42, "user@example.com", "", "Бриф", "", "", 100000)
	snap := original.Snapshot()

	if snap.AccessToken == "" {
		t.Error("Snapshot должен содержать AccessToken")
	}

	restored := RestoreOrder(snap)
	if restored.AccessToken() != original.AccessToken() {
		t.Errorf("токен потерялся при Snapshot/Restore: было %q, стало %q",
			original.AccessToken(), restored.AccessToken())
	}
	if restored.ID() != original.ID() {
		t.Error("ID не совпадает после Restore")
	}
}

// --- helpers ---

func newTestOrder(t *testing.T) *Order {
	t.Helper()
	o, err := NewOrder(1, "user@example.com", "", "Тестовый бриф", "", "", 150000)
	if err != nil {
		t.Fatalf("не удалось создать тестовый заказ: %v", err)
	}
	return o
}

func newPaidOrder(t *testing.T) *Order {
	t.Helper()
	o := newTestOrder(t)
	if err := o.MarkPaid(); err != nil {
		t.Fatalf("MarkPaid упал: %v", err)
	}
	return o
}

func newProcessingOrder(t *testing.T) *Order {
	t.Helper()
	o := newPaidOrder(t)
	_ = o.Enqueue()
	_ = o.StartProcessing(uuid.New())
	return o
}
