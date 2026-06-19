package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// AccountStatus описывает текущее состояние Suno-аккаунта в пуле ресурсов системы.
type AccountStatus string

const (
	// AccountStatusActive - аккаунт в ротации; занятость определяется счётчиком
	// одновременных задач concurrentTasks, а не статусом.
	AccountStatusActive AccountStatus = "active"
	// AccountStatusBusy устарел: занятость теперь моделируется через concurrentTasks
	// и maxConcurrentTasks. Константа сохранена для совместимости со старыми
	// записями в БД (миграция 0002 переводит такие строки обратно в active).
	//
	// Deprecated: используйте слоты (AcquireSlot/ReleaseSlot) вместо статуса Busy.
	AccountStatusBusy AccountStatus = "busy"
	// AccountStatusCooldown - аккаунт временно выведен из ротации (например, после rate-limit от Suno).
	AccountStatusCooldown AccountStatus = "cooldown"
	// AccountStatusOutOfTokens - на аккаунте закончились токены/кредиты генерации.
	AccountStatusOutOfTokens AccountStatus = "out_of_tokens"
	// AccountStatusBanned - аккаунт заблокирован площадкой Suno, выведен из пула навсегда.
	AccountStatusBanned AccountStatus = "banned"
)

// DefaultMaxConcurrentTasks — сколько генераций аккаунт обслуживает параллельно,
// если иное не задано. 1 соответствует прежнему поведению (эксклюзивный захват);
// для платных аккаунтов Suno (Pro/Premier) значение можно увеличить.
const DefaultMaxConcurrentTasks = 1

// Доменные ошибки, связанные с жизненным циклом аккаунта
var (
	ErrAccountNotFound          = errors.New("аккаунт не найден")
	ErrNoAvailableAccount       = errors.New("в пуле нет свободных аккаунтов Suno")
	ErrInsufficientTokenBalance = errors.New("недостаточно токенов на аккаунте")
	ErrInvalidAccountTransition = errors.New("недопустимый переход статуса аккаунта")
)

// SunoAccount - агрегат, представляющий один аккаунт Suno, используемый
// Numaestra как ресурс пула для генерации музыки.
// Сессия (cookies/local storage), необходимая anti-detect клиенту, хранится
// в зашифрованном виде и никогда не должна попадать в логи или ответы API.
type SunoAccount struct {
	id     uuid.UUID
	status AccountStatus

	email            string
	encryptedSession string

	tokenBalance int // оставшиеся кредиты генерации
	failureCount int // счётчик последовательных неудач, для эскалации в Banned

	// maxConcurrentTasks — сколько генераций аккаунт может вести одновременно.
	// concurrentTasks — сколько ведёт прямо сейчас. Аккаунт доступен, пока
	// concurrentTasks < maxConcurrentTasks. Это снимает узкое место «один аккаунт —
	// одна генерация» для платных аккаунтов Suno.
	maxConcurrentTasks int
	concurrentTasks    int

	cooldownUntil *time.Time
	lastUsedAt    *time.Time
	createdAt     time.Time
	updatedAt     time.Time
}

// NewSunoAccount создаёт новый аккаунт в пуле со статусом Active.
func NewSunoAccount(email, encryptedSession string, tokenBalance int) (*SunoAccount, error) {
	if email == "" {
		return nil, errors.New("email аккаунта не может быть пустым")
	}
	if encryptedSession == "" {
		return nil, errors.New("сессия аккаунта не может быть пустой")
	}
	if tokenBalance < 0 {
		return nil, errors.New("баланс токенов не может быть отрицательным")
	}

	now := time.Now().UTC()
	return &SunoAccount{
		id:                 uuid.New(),
		status:             AccountStatusActive,
		email:              email,
		encryptedSession:   encryptedSession,
		tokenBalance:       tokenBalance,
		maxConcurrentTasks: DefaultMaxConcurrentTasks,
		createdAt:          now,
		updatedAt:          now,
	}, nil
}

// SetMaxConcurrentTasks задаёт лимит одновременных генераций (>=1).
func (a *SunoAccount) SetMaxConcurrentTasks(n int) error {
	if n < 1 {
		return errors.New("лимит одновременных задач должен быть не меньше 1")
	}
	a.maxConcurrentTasks = n
	a.touch()
	return nil
}

// SunoAccountSnapshot - сырые данные из хранилища для восстановления агрегата.
// Используется исключительно мапперами репозитория, не доменной логикой.
type SunoAccountSnapshot struct {
	ID                 uuid.UUID
	Email              string
	EncryptedSession   string
	Status             AccountStatus
	TokenBalance       int
	FailureCount       int
	MaxConcurrentTasks int
	ConcurrentTasks    int
	CooldownUntil      *time.Time
	LastUsedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// RestoreSunoAccount восстанавливает агрегат из снапшота хранилища.
func RestoreSunoAccount(s SunoAccountSnapshot) *SunoAccount {
	maxConcurrent := s.MaxConcurrentTasks
	if maxConcurrent < 1 {
		// Подстраховка для легаси-строк без значения: ведём себя как раньше.
		maxConcurrent = DefaultMaxConcurrentTasks
	}
	return &SunoAccount{
		id:                 s.ID,
		email:              s.Email,
		encryptedSession:   s.EncryptedSession,
		status:             s.Status,
		tokenBalance:       s.TokenBalance,
		failureCount:       s.FailureCount,
		maxConcurrentTasks: maxConcurrent,
		concurrentTasks:    s.ConcurrentTasks,
		cooldownUntil:      s.CooldownUntil,
		lastUsedAt:         s.LastUsedAt,
		createdAt:          s.CreatedAt,
		updatedAt:          s.UpdatedAt,
	}
}

// --- Геттеры ---
func (a *SunoAccount) ID() uuid.UUID             { return a.id }
func (a *SunoAccount) Status() AccountStatus     { return a.status }
func (a *SunoAccount) Email() string             { return a.email }
func (a *SunoAccount) EncryptedSession() string  { return a.encryptedSession }
func (a *SunoAccount) TokenBalance() int         { return a.tokenBalance }
func (a *SunoAccount) FailureCount() int          { return a.failureCount }
func (a *SunoAccount) MaxConcurrentTasks() int    { return a.maxConcurrentTasks }
func (a *SunoAccount) ConcurrentTasks() int       { return a.concurrentTasks }
func (a *SunoAccount) CooldownUntil() *time.Time  { return a.cooldownUntil }
func (a *SunoAccount) UpdatedAt() time.Time       { return a.updatedAt }

// IsAvailable проверяет, может ли аккаунт прямо сейчас принять ещё одну задачу:
// он активен, есть токены, не в cooldown и не достигнут лимит одновременных задач.
func (a *SunoAccount) IsAvailable(now time.Time) bool {
	if a.status != AccountStatusActive {
		return false
	}
	if a.tokenBalance <= 0 {
		return false
	}
	if a.cooldownUntil != nil && now.Before(*a.cooldownUntil) {
		return false
	}
	if a.concurrentTasks >= a.maxConcurrentTasks {
		return false
	}
	return true
}

// AcquireSlot занимает один слот одновременной генерации. Вызывается строго
// внутри атомарной операции захвата (см. AccountRepository.FetchAndLockAvailable):
// SELECT ... FOR UPDATE SKIP LOCKED гарантирует, что инкремент счётчика не
// потеряется при параллельном захвате. Статус остаётся Active — занятость
// выражается счётчиком, а не статусом.
func (a *SunoAccount) AcquireSlot(now time.Time) error {
	if !a.IsAvailable(now) {
		return ErrInvalidAccountTransition
	}
	a.concurrentTasks++
	a.lastUsedAt = &now
	a.touch()
	return nil
}

// ReleaseSlot освобождает один слот после завершения задачи (успешного или нет).
// Статус не меняется (в частности, не перетирает Banned/Cooldown), что важно
// для корректного порядка с RegisterFailure.
func (a *SunoAccount) ReleaseSlot() {
	if a.concurrentTasks > 0 {
		a.concurrentTasks--
	}
	now := time.Now().UTC()
	a.lastUsedAt = &now
	a.touch()
}

// ConsumeTokens списывает потраченные токены после успешной генерации
func (a *SunoAccount) ConsumeTokens(amount int) error {
	if amount < 0 {
		return errors.New("количество списываемых токенов не может быть отрицательным")
	}
	if a.tokenBalance < amount {
		return ErrInsufficientTokenBalance
	}
	a.tokenBalance -= amount
	if a.tokenBalance == 0 {
		a.status = AccountStatusOutOfTokens
	}
	a.touch()
	return nil
}

// EnterCooldown временно выводит аккаунт из ротации (например, после rate-limit от Suno).
func (a *SunoAccount) EnterCooldown(duration time.Duration) {
	until := time.Now().UTC().Add(duration)
	a.status = AccountStatusCooldown
	a.cooldownUntil = &until
	a.touch()
}

// RegisterFailure увеличивает счётчик ошибок и при превышении порога эскалирует в Banned.
// Порог приходит снаружи (из политики use-case'а), чтобы домен не зависел от конкретных констант.
func (a *SunoAccount) RegisterFailure(threshold int) {
	a.failureCount++
	if a.failureCount >= threshold {
		a.status = AccountStatusBanned
	}
	a.touch()
}

// ResetFailures сбрасывает счётчик ошибок после успешного использования.
func (a *SunoAccount) ResetFailures() {
	a.failureCount = 0
	a.touch()
}

func (a *SunoAccount) touch() {
	a.updatedAt = time.Now().UTC()
}

// AccountRepository - контракт для работы с хранилищем аккаунтов Suno.
type AccountRepository interface {
	// FetchAndLockAvailable атомарно находит и блокирует один свободный аккаунт
	// (в PostgreSQL-реализации - через "SELECT ... FOR UPDATE SKIP LOCKED" в транзакции),
	// переводит его в статус Busy и возвращает вызывающей стороне. Это критическая
	// операция для предотвращения race condition при параллельных запросах на генерацию.
	// Если свободных аккаунтов нет, возвращает ErrNoAvailableAccount.
	FetchAndLockAvailable(ctx context.Context) (*SunoAccount, error)

	GetByID(ctx context.Context, id uuid.UUID) (*SunoAccount, error)
	Create(ctx context.Context, account *SunoAccount) error
	Update(ctx context.Context, account *SunoAccount) error
	ListByStatus(ctx context.Context, status AccountStatus) ([]*SunoAccount, error)

	// List возвращает все аккаунты пула без фильтрации (для Admin API).
	List(ctx context.Context) ([]*SunoAccount, error)
	// SetStatus атомарно меняет статус аккаунта (Admin API: активация, блокировка и т.д.).
	SetStatus(ctx context.Context, id uuid.UUID, status AccountStatus) error
}

func (a *SunoAccount) Snapshot() SunoAccountSnapshot {
	return SunoAccountSnapshot{
		ID:                 a.id,
		Email:              a.email,
		EncryptedSession:   a.encryptedSession,
		Status:             a.status,
		TokenBalance:       a.tokenBalance,
		FailureCount:       a.failureCount,
		MaxConcurrentTasks: a.maxConcurrentTasks,
		ConcurrentTasks:    a.concurrentTasks,
		CooldownUntil:      a.cooldownUntil,
		LastUsedAt:         a.lastUsedAt,
		CreatedAt:          a.createdAt,
		UpdatedAt:          a.updatedAt,
	}
}
