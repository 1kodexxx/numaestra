package domain

import (
	"testing"
	"time"
)

// --- NewSunoAccount ---

func TestNewSunoAccount_Valid(t *testing.T) {
	a, err := NewSunoAccount("test@suno.ai", "encrypted-session-data", 500)
	if err != nil {
		t.Fatalf("ожидали успех, получили ошибку: %v", err)
	}
	if a.Status() != AccountStatusActive {
		t.Errorf("новый аккаунт должен быть Active, получили %q", a.Status())
	}
	if a.TokenBalance() != 500 {
		t.Errorf("ожидали баланс 500, получили %d", a.TokenBalance())
	}
}

func TestNewSunoAccount_RequiresEmail(t *testing.T) {
	_, err := NewSunoAccount("", "session", 100)
	if err == nil {
		t.Error("ожидали ошибку при пустом email")
	}
}

func TestNewSunoAccount_RequiresSession(t *testing.T) {
	_, err := NewSunoAccount("test@suno.ai", "", 100)
	if err == nil {
		t.Error("ожидали ошибку при пустой сессии")
	}
}

func TestNewSunoAccount_NegativeBalance(t *testing.T) {
	_, err := NewSunoAccount("test@suno.ai", "session", -1)
	if err == nil {
		t.Error("ожидали ошибку при отрицательном балансе")
	}
}

// --- MarkBusy / Release ---

func TestAccount_MarkBusy(t *testing.T) {
	a := newTestAccount(t)
	if err := a.MarkBusy(); err != nil {
		t.Fatalf("MarkBusy упал: %v", err)
	}
	if a.Status() != AccountStatusBusy {
		t.Errorf("ожидали Busy, получили %q", a.Status())
	}
}

func TestAccount_MarkBusy_OnlyFromActive(t *testing.T) {
	a := newTestAccount(t)
	_ = a.MarkBusy()

	err := a.MarkBusy() // повторный вызов из Busy
	if err != ErrInvalidAccountTransition {
		t.Errorf("MarkBusy из Busy должен вернуть ErrInvalidAccountTransition, получили: %v", err)
	}
}

func TestAccount_Release_FromBusy(t *testing.T) {
	a := newTestAccount(t)
	_ = a.MarkBusy()
	a.Release()

	if a.Status() != AccountStatusActive {
		t.Errorf("после Release из Busy ожидали Active, получили %q", a.Status())
	}
}

func TestAccount_Release_DoesNotOverwriteBanned(t *testing.T) {
	// Это ключевой тест для проблемы #1 которую мы исправляли:
	// RegisterFailure должен вызываться ДО Release, иначе Release перетрёт Banned на Active.
	a := newTestAccount(t)
	_ = a.MarkBusy()

	// Правильный порядок: сначала регистрируем ошибку, потом Release
	a.RegisterFailure(1) // порог 1 → сразу Banned
	a.Release()          // Release не должен перетереть Banned

	if a.Status() != AccountStatusBanned {
		t.Errorf("Release не должен перетирать Banned: ожидали Banned, получили %q", a.Status())
	}
}

func TestAccount_Release_WrongOrder_WouldBeBug(t *testing.T) {
	// Демонстрация бага: Release → RegisterFailure в неправильном порядке.
	// Этот тест фиксирует ПРАВИЛЬНОЕ поведение домена:
	// Release сначала ставит Active, потом RegisterFailure ставит Banned — итог всё равно Banned.
	// Баг был в usecase (не в домене), и мы его уже исправили.
	a := newTestAccount(t)
	_ = a.MarkBusy()

	a.Release()          // Active
	a.RegisterFailure(1) // Banned

	if a.Status() != AccountStatusBanned {
		t.Errorf("после Release+RegisterFailure ожидали Banned, получили %q", a.Status())
	}
}

// --- RegisterFailure ---

func TestAccount_RegisterFailure_BelowThreshold(t *testing.T) {
	a := newTestAccount(t)
	a.RegisterFailure(3) // failureCount=1, порог=3 → не Banned
	if a.Status() == AccountStatusBanned {
		t.Error("один сбой при пороге 3 не должен банить аккаунт")
	}
}

func TestAccount_RegisterFailure_AtThreshold(t *testing.T) {
	a := newTestAccount(t)
	a.RegisterFailure(3)
	a.RegisterFailure(3)
	a.RegisterFailure(3) // failureCount=3, порог=3 → Banned

	if a.Status() != AccountStatusBanned {
		t.Errorf("при достижении порога ожидали Banned, получили %q", a.Status())
	}
}

func TestAccount_ResetFailures(t *testing.T) {
	a := newTestAccount(t)
	a.RegisterFailure(3)
	a.RegisterFailure(3)
	a.ResetFailures()
	a.RegisterFailure(3) // счётчик сброшен → снова 1 из 3

	if a.Status() == AccountStatusBanned {
		t.Error("после ResetFailures счётчик должен обнулиться")
	}
}

// --- ConsumeTokens ---

func TestAccount_ConsumeTokens(t *testing.T) {
	a := newTestAccount(t) // 100 токенов
	if err := a.ConsumeTokens(40); err != nil {
		t.Fatalf("ConsumeTokens упал: %v", err)
	}
	if a.TokenBalance() != 60 {
		t.Errorf("ожидали 60 токенов, получили %d", a.TokenBalance())
	}
}

func TestAccount_ConsumeTokens_InsufficientBalance(t *testing.T) {
	a := newTestAccount(t) // 100 токенов
	err := a.ConsumeTokens(200)
	if err != ErrInsufficientTokenBalance {
		t.Errorf("ожидали ErrInsufficientTokenBalance, получили: %v", err)
	}
}

func TestAccount_ConsumeTokens_ToZero_SetsOutOfTokens(t *testing.T) {
	a := newTestAccount(t) // 100 токенов
	if err := a.ConsumeTokens(100); err != nil {
		t.Fatalf("ConsumeTokens до нуля упал: %v", err)
	}
	if a.Status() != AccountStatusOutOfTokens {
		t.Errorf("при нулевом балансе ожидали OutOfTokens, получили %q", a.Status())
	}
}

// --- EnterCooldown ---

func TestAccount_EnterCooldown(t *testing.T) {
	a := newTestAccount(t)
	a.EnterCooldown(5 * time.Minute)

	if a.Status() != AccountStatusCooldown {
		t.Errorf("ожидали Cooldown, получили %q", a.Status())
	}
}

func TestAccount_IsAvailable(t *testing.T) {
	a := newTestAccount(t)
	now := time.Now().UTC()
	if !a.IsAvailable(now) {
		t.Error("новый Active аккаунт с токенами должен быть доступен")
	}

	a.EnterCooldown(5 * time.Minute)
	if a.IsAvailable(now) {
		t.Error("аккаунт в Cooldown не должен быть доступен")
	}
}

// --- helpers ---

func newTestAccount(t *testing.T) *SunoAccount {
	t.Helper()
	a, err := NewSunoAccount("test@suno.ai", "encrypted-session", 100)
	if err != nil {
		t.Fatalf("не удалось создать тестовый аккаунт: %v", err)
	}
	return a
}
