package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

var errDummy = errors.New("сбой внешнего сервиса")

// closed → normal operation

func TestBreaker_Closed_SuccessPassesThrough(t *testing.T) {
	b := New("svc", 3, time.Second)
	called := false
	if err := b.Do(func() error { called = true; return nil }); err != nil {
		t.Fatalf("ожидали nil, получили: %v", err)
	}
	if !called {
		t.Fatal("fn должна была вызваться")
	}
	if b.State() != "closed" {
		t.Errorf("ожидали closed, получили %q", b.State())
	}
}

func TestBreaker_Closed_ErrorPassesThrough(t *testing.T) {
	b := New("svc", 3, time.Second)
	err := b.Do(func() error { return errDummy })
	if !errors.Is(err, errDummy) {
		t.Fatalf("ожидали errDummy, получили: %v", err)
	}
	// Один сбой не должен размыкать цепь (порог = 3).
	if b.State() != "closed" {
		t.Errorf("один сбой не должен размыкать: %q", b.State())
	}
}

// closed → open (достигнут порог)

func TestBreaker_OpensAfterThreshold(t *testing.T) {
	b := New("svc", 3, time.Second)
	for i := 0; i < 3; i++ {
		_ = b.Do(func() error { return errDummy })
	}
	if b.State() != "open" {
		t.Errorf("после %d ошибок ожидали open, получили %q", 3, b.State())
	}
}

// open → отклоняет запросы без вызова fn

func TestBreaker_Open_RejectsWithoutCallingFn(t *testing.T) {
	b := New("svc", 1, time.Hour)
	_ = b.Do(func() error { return errDummy }) // открываем

	called := false
	err := b.Do(func() error { called = true; return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("ожидали ErrCircuitOpen, получили: %v", err)
	}
	if called {
		t.Fatal("fn не должна вызываться, пока цепь разомкнута")
	}
}

// open → half-open после истечения таймаута

func TestBreaker_HalfOpen_AfterTimeout(t *testing.T) {
	b := New("svc", 1, 10*time.Millisecond)
	_ = b.Do(func() error { return errDummy }) // открываем

	time.Sleep(20 * time.Millisecond)

	// Первый запрос после таймаута должен пройти (half-open пробный запрос).
	called := false
	if err := b.Do(func() error { called = true; return nil }); err != nil {
		t.Fatalf("пробный запрос не должен отклоняться: %v", err)
	}
	if !called {
		t.Fatal("fn должна вызваться в half-open")
	}
	// Успешный пробный запрос → Closed.
	if b.State() != "closed" {
		t.Errorf("после успеха в half-open ожидали closed, получили %q", b.State())
	}
}

// half-open + ошибка → снова open

func TestBreaker_HalfOpen_FailureReopens(t *testing.T) {
	b := New("svc", 1, 10*time.Millisecond)
	_ = b.Do(func() error { return errDummy }) // открываем

	time.Sleep(20 * time.Millisecond)

	err := b.Do(func() error { return errDummy }) // провал в half-open
	if !errors.Is(err, errDummy) {
		t.Fatalf("ожидали errDummy из half-open, получили: %v", err)
	}
	if b.State() != "open" {
		t.Errorf("после провала в half-open ожидали open, получили %q", b.State())
	}
}

// успех сбрасывает счётчик ошибок

func TestBreaker_SuccessResetsFailureCount(t *testing.T) {
	b := New("svc", 3, time.Second)
	// Два подряд сбоя.
	_ = b.Do(func() error { return errDummy })
	_ = b.Do(func() error { return errDummy })
	// Успех должен сбросить счётчик.
	_ = b.Do(func() error { return nil })
	// Ещё два сбоя — цепь не должна открыться (счётчик сброшен до 0).
	_ = b.Do(func() error { return errDummy })
	_ = b.Do(func() error { return errDummy })
	if b.State() != "closed" {
		t.Errorf("счётчик должен был сброситься; ожидали closed, получили %q", b.State())
	}
}
