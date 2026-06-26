package demolimit

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newTestLimiter(t *testing.T, dailyLimit, perEmailHours int) (*Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return New(rdb, dailyLimit, perEmailHours), mr
}

// Один email получает демо один раз; второй заказ с тем же email — отказ.
func TestReserve_PerEmail_BlocksSecondOrder(t *testing.T) {
	l, _ := newTestLimiter(t, 0, 24)
	ctx := context.Background()

	ok, err := l.Reserve(ctx, uuid.New(), "User@Example.com")
	if err != nil || !ok {
		t.Fatalf("первый заказ должен пройти: ok=%v err=%v", ok, err)
	}
	// Тот же email (другой регистр) другим заказом — отказ.
	ok, err = l.Reserve(ctx, uuid.New(), "user@example.com")
	if err != nil {
		t.Fatalf("ошибка: %v", err)
	}
	if ok {
		t.Error("второй заказ с тем же email должен быть отклонён")
	}
}

// Повтор того же заказа (retry задачи) проходит, не тратя лимит.
func TestReserve_SameOrder_Idempotent(t *testing.T) {
	l, _ := newTestLimiter(t, 0, 24)
	ctx := context.Background()
	id := uuid.New()

	ok1, _ := l.Reserve(ctx, id, "a@example.com")
	ok2, err := l.Reserve(ctx, id, "a@example.com")
	if err != nil {
		t.Fatalf("ошибка: %v", err)
	}
	if !ok1 || !ok2 {
		t.Errorf("повтор того же заказа должен проходить: ok1=%v ok2=%v", ok1, ok2)
	}
}

// Дневной лимит: после исчерпания новые заказы отклоняются.
func TestReserve_DailyLimit_Blocks(t *testing.T) {
	l, _ := newTestLimiter(t, 2, 0) // лимит 2, email-лимит выключен
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		ok, err := l.Reserve(ctx, uuid.New(), "")
		if err != nil || !ok {
			t.Fatalf("заказ %d в пределах лимита должен пройти: ok=%v err=%v", i, ok, err)
		}
	}
	ok, err := l.Reserve(ctx, uuid.New(), "")
	if err != nil {
		t.Fatalf("ошибка: %v", err)
	}
	if ok {
		t.Error("заказ сверх дневного лимита должен быть отклонён")
	}
}

// При превышении дневного лимита занятый email-ключ откатывается — тот же email
// сможет получить демо завтра (ключ не «сгорел» впустую).
func TestReserve_DailyLimitExceeded_RollsBackEmail(t *testing.T) {
	l, mr := newTestLimiter(t, 1, 24)
	ctx := context.Background()

	ok, _ := l.Reserve(ctx, uuid.New(), "first@example.com")
	if !ok {
		t.Fatal("первый заказ должен пройти")
	}
	// Второй email упирается в дневной лимит (1) → отказ и откат email-ключа.
	ok, _ = l.Reserve(ctx, uuid.New(), "second@example.com")
	if ok {
		t.Fatal("второй заказ должен упереться в дневной лимит")
	}
	if mr.Exists(emailKey("second@example.com")) {
		t.Error("email-ключ должен быть откачен при превышении дневного лимита")
	}
}

// Нулевые лимиты отключают защиту — всё проходит.
func TestReserve_Disabled_AllowsAll(t *testing.T) {
	l, _ := newTestLimiter(t, 0, 0)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		ok, err := l.Reserve(ctx, uuid.New(), "x@example.com")
		if err != nil || !ok {
			t.Fatalf("без лимитов всё должно проходить: ok=%v err=%v", ok, err)
		}
	}
}
