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

// --- AcquireSlot / ReleaseSlot ---

func TestAccount_AcquireSlot(t *testing.T) {
	a := newTestAccount(t)
	if err := a.AcquireSlot(time.Now().UTC()); err != nil {
		t.Fatalf("AcquireSlot упал: %v", err)
	}
	if a.ConcurrentTasks() != 1 {
		t.Errorf("ожидали 1 занятый слот, получили %d", a.ConcurrentTasks())
	}
	// Статус НЕ меняется на busy — занятость выражается счётчиком.
	if a.Status() != AccountStatusActive {
		t.Errorf("статус должен оставаться active, получили %q", a.Status())
	}
}

func TestAccount_AcquireSlot_RespectsMaxConcurrency(t *testing.T) {
	a := newTestAccount(t) // по умолчанию max=1
	if err := a.AcquireSlot(time.Now().UTC()); err != nil {
		t.Fatalf("первый захват слота должен пройти: %v", err)
	}
	// Лимит исчерпан (1 из 1) — второй захват должен быть отклонён.
	if err := a.AcquireSlot(time.Now().UTC()); err != ErrInvalidAccountTransition {
		t.Errorf("захват сверх лимита должен вернуть ErrInvalidAccountTransition, получили %v", err)
	}
}

func TestAccount_AcquireSlot_ParallelUpToLimit(t *testing.T) {
	a := newTestAccount(t)
	if err := a.SetMaxConcurrentTasks(3); err != nil {
		t.Fatalf("SetMaxConcurrentTasks упал: %v", err)
	}
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if err := a.AcquireSlot(now); err != nil {
			t.Fatalf("слот %d должен захватываться при лимите 3: %v", i+1, err)
		}
	}
	if a.ConcurrentTasks() != 3 {
		t.Errorf("ожидали 3 занятых слота, получили %d", a.ConcurrentTasks())
	}
	if a.IsAvailable(now) {
		t.Error("при исчерпанном лимите аккаунт не должен быть доступен")
	}
}

func TestAccount_ReleaseSlot(t *testing.T) {
	a := newTestAccount(t)
	_ = a.AcquireSlot(time.Now().UTC())
	a.ReleaseSlot()

	if a.ConcurrentTasks() != 0 {
		t.Errorf("после ReleaseSlot ожидали 0 занятых слотов, получили %d", a.ConcurrentTasks())
	}
	if !a.IsAvailable(time.Now().UTC()) {
		t.Error("после освобождения слота аккаунт снова должен быть доступен")
	}
}

func TestAccount_ReleaseSlot_DoesNotOverwriteBanned(t *testing.T) {
	// Ключевой инвариант: ReleaseSlot не меняет статус, поэтому не перетирает
	// Banned обратно в active — независимо от порядка с RegisterFailure.
	a := newTestAccount(t)
	_ = a.AcquireSlot(time.Now().UTC())

	a.RegisterFailure(1) // порог 1 → сразу Banned
	a.ReleaseSlot()

	if a.Status() != AccountStatusBanned {
		t.Errorf("ReleaseSlot не должен перетирать Banned: ожидали Banned, получили %q", a.Status())
	}
	if a.ConcurrentTasks() != 0 {
		t.Errorf("слот должен освободиться даже у забаненного аккаунта, занято=%d", a.ConcurrentTasks())
	}
}

func TestAccount_ReleaseSlot_NeverNegative(t *testing.T) {
	a := newTestAccount(t)
	a.ReleaseSlot() // лишний вызов без захвата не должен уводить счётчик в минус
	if a.ConcurrentTasks() != 0 {
		t.Errorf("счётчик слотов не должен становиться отрицательным, получили %d", a.ConcurrentTasks())
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

// --- Throttle (самовосстанавливающаяся пауза) ---

func TestAccount_Throttle_KeepsActiveButUnavailable(t *testing.T) {
	a := newTestAccount(t)
	a.Throttle(5 * time.Minute)

	// Статус остаётся Active — это принципиально для авто-восстановления через SQL/IsAvailable.
	if a.Status() != AccountStatusActive {
		t.Errorf("после Throttle статус должен остаться Active, получили %q", a.Status())
	}
	if a.CooldownUntil() == nil {
		t.Fatal("после Throttle CooldownUntil не должен быть nil")
	}
	// Во время паузы аккаунт недоступен...
	if a.IsAvailable(time.Now().UTC()) {
		t.Error("во время паузы аккаунт не должен быть доступен")
	}
	// ...а после её истечения снова доступен без ручного вмешательства.
	if !a.IsAvailable(time.Now().UTC().Add(6 * time.Minute)) {
		t.Error("после истечения паузы аккаунт должен сам вернуться в ротацию")
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

// --- RestoreSunoAccount / Snapshot ---

func TestRestoreSunoAccount_Roundtrip(t *testing.T) {
	original := newTestAccount(t)
	_ = original.AcquireSlot(time.Now().UTC())
	original.RegisterFailure(5)

	snap := original.Snapshot()
	restored := RestoreSunoAccount(snap)

	if restored.ID() != original.ID() {
		t.Errorf("ID: got %v, want %v", restored.ID(), original.ID())
	}
	if restored.Email() != original.Email() {
		t.Errorf("Email: got %q, want %q", restored.Email(), original.Email())
	}
	if restored.EncryptedSession() != original.EncryptedSession() {
		t.Errorf("EncryptedSession mismatch")
	}
	if restored.Status() != original.Status() {
		t.Errorf("Status: got %q, want %q", restored.Status(), original.Status())
	}
	if restored.TokenBalance() != original.TokenBalance() {
		t.Errorf("TokenBalance: got %d, want %d", restored.TokenBalance(), original.TokenBalance())
	}
	if restored.FailureCount() != original.FailureCount() {
		t.Errorf("FailureCount: got %d, want %d", restored.FailureCount(), original.FailureCount())
	}
	if restored.ConcurrentTasks() != original.ConcurrentTasks() {
		t.Errorf("ConcurrentTasks: got %d, want %d", restored.ConcurrentTasks(), original.ConcurrentTasks())
	}
	if restored.MaxConcurrentTasks() != original.MaxConcurrentTasks() {
		t.Errorf("MaxConcurrentTasks: got %d, want %d", restored.MaxConcurrentTasks(), original.MaxConcurrentTasks())
	}
}

func TestRestoreSunoAccount_ZeroMaxConcurrent_DefaultsToOne(t *testing.T) {
	snap := domain_sunoAccountSnap(0)
	a := RestoreSunoAccount(snap)
	if a.MaxConcurrentTasks() != DefaultMaxConcurrentTasks {
		t.Errorf("MaxConcurrentTasks с нулём должен принять дефолт %d, получили %d",
			DefaultMaxConcurrentTasks, a.MaxConcurrentTasks())
	}
}

// --- Атомарные дельты счётчиков (фикс гонки concurrent_tasks/token_balance) ---

// Снапшот должен отдавать ИЗМЕНЕНИЕ счётчиков относительно загруженного состояния
// (дельту), а не абсолют — репозиторий применит её атомарно (col = col + delta),
// чтобы конкурентные релизы одной строки не теряли декремент.
func TestSnapshot_CounterDeltas_RelativeToLoadedState(t *testing.T) {
	snap := domain_sunoAccountSnap(3)
	snap.TokenBalance = 5
	snap.ConcurrentTasks = 2
	snap.FailureCount = 1
	a := RestoreSunoAccount(snap)

	a.ReleaseSlot()        // 2 -> 1
	_ = a.ConsumeTokens(1) // 5 -> 4
	a.RegisterFailure(10)  // 1 -> 2

	out := a.Snapshot()
	if out.ConcurrentTasksDelta != -1 {
		t.Errorf("ConcurrentTasksDelta: ожидали -1, получили %d", out.ConcurrentTasksDelta)
	}
	if out.TokenBalanceDelta != -1 {
		t.Errorf("TokenBalanceDelta: ожидали -1, получили %d", out.TokenBalanceDelta)
	}
	if out.FailureCountDelta != 1 {
		t.Errorf("FailureCountDelta: ожидали +1, получили %d", out.FailureCountDelta)
	}
	// Абсолюты тоже на месте (для Create/захвата слота).
	if out.ConcurrentTasks != 1 || out.TokenBalance != 4 {
		t.Errorf("абсолютные значения: ct=%d tb=%d", out.ConcurrentTasks, out.TokenBalance)
	}
}

// Rebaseline переустанавливает базу: после неё дельты нулевые, а следующая мутация
// формирует дельту заново — без удвоения уже зафиксированного изменения.
func TestRebaseline_ResetsDeltas(t *testing.T) {
	snap := domain_sunoAccountSnap(3)
	snap.ConcurrentTasks = 2
	a := RestoreSunoAccount(snap)

	a.ReleaseSlot() // delta -1
	a.Rebaseline()  // фиксируем -> база = текущее (1)
	if d := a.Snapshot().ConcurrentTasksDelta; d != 0 {
		t.Fatalf("после Rebaseline дельта должна обнулиться, получили %d", d)
	}

	a.ReleaseSlot() // 1 -> 0, новая дельта -1
	if d := a.Snapshot().ConcurrentTasksDelta; d != -1 {
		t.Errorf("после повторного релиза ожидали дельту -1, получили %d", d)
	}
}

// Сценарий захвата-затем-освобождения (как FetchAndLockAvailable → Update): после
// фиксации захвата (Rebaseline) последующее освобождение даёт дельту -1, корректно
// отменяя +1. Без Rebaseline release дал бы дельту 0 и слот «утёк» бы навсегда.
func TestAcquireRebaselineRelease_NetsToCorrectDelta(t *testing.T) {
	snap := domain_sunoAccountSnap(3)
	snap.ConcurrentTasks = 0
	a := RestoreSunoAccount(snap)

	_ = a.AcquireSlot(time.Now().UTC()) // 0 -> 1
	if d := a.Snapshot().ConcurrentTasksDelta; d != 1 {
		t.Fatalf("после захвата ожидали дельту +1, получили %d", d)
	}
	a.Rebaseline() // FetchAndLockAvailable зафиксировал захват абсолютно

	a.ReleaseSlot() // 1 -> 0
	if d := a.Snapshot().ConcurrentTasksDelta; d != -1 {
		t.Errorf("освобождение после захвата должно дать дельту -1, получили %d", d)
	}
}

func TestSunoAccount_CooldownUntil_Getter(t *testing.T) {
	a := newTestAccount(t)
	if a.CooldownUntil() != nil {
		t.Error("у нового аккаунта CooldownUntil должен быть nil")
	}
	a.EnterCooldown(5 * time.Minute)
	if a.CooldownUntil() == nil {
		t.Error("после EnterCooldown CooldownUntil не должен быть nil")
	}
}

func TestSunoAccount_UpdatedAt_ChangesOnMutation(t *testing.T) {
	a := newTestAccount(t)
	before := a.UpdatedAt()
	time.Sleep(time.Millisecond) // гарантируем разницу времени
	_ = a.AcquireSlot(time.Now().UTC())
	if !a.UpdatedAt().After(before) {
		t.Error("UpdatedAt должен обновляться при мутации")
	}
}

// --- helpers ---

// domain_sunoAccountSnap возвращает снапшот с заданным maxConcurrentTasks.
func domain_sunoAccountSnap(maxConcurrent int) SunoAccountSnapshot {
	return SunoAccountSnapshot{
		Email:              "test@suno.ai",
		EncryptedSession:   "session",
		Status:             AccountStatusActive,
		TokenBalance:       100,
		MaxConcurrentTasks: maxConcurrent,
	}
}

func newTestAccount(t *testing.T) *SunoAccount {
	t.Helper()
	a, err := NewSunoAccount("test@suno.ai", "encrypted-session", 100)
	if err != nil {
		t.Fatalf("не удалось создать тестовый аккаунт: %v", err)
	}
	return a
}
