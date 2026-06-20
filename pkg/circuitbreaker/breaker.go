// Package circuitbreaker реализует паттерн Circuit Breaker для защиты от
// лавинообразных сбоев при недоступности внешних сервисов (Suno API, LLM).
//
// Состояния автомата:
//
//	Closed   → нормальная работа; при достижении порога ошибок → Open.
//	Open     → запросы отклоняются сразу (ErrCircuitOpen); по истечении
//	           timeout → Half-Open для пробного запроса.
//	Half-Open → один пробный запрос; успех → Closed, ошибка → Open снова.
package circuitbreaker

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrCircuitOpen возвращается, когда автомат разомкнут и запрос отклонён.
var ErrCircuitOpen = errors.New("circuit breaker разомкнут: внешний сервис временно недоступен")

type state int

const (
	stateClosed   state = iota
	stateOpen
	stateHalfOpen
)

// Breaker — потокобезопасный circuit breaker.
type Breaker struct {
	mu sync.Mutex

	name      string
	threshold int           // число последовательных ошибок до размыкания
	timeout   time.Duration // время ожидания в Open перед переходом в Half-Open

	failures    int
	state       state
	openedAt    time.Time
}

// New создаёт новый Breaker.
//   - name      — имя для логирования (например, "suno", "llm")
//   - threshold — число последовательных ошибок до размыкания цепи
//   - timeout   — как долго оставаться в Open до пробного запроса
func New(name string, threshold int, timeout time.Duration) *Breaker {
	return &Breaker{
		name:      name,
		threshold: threshold,
		timeout:   timeout,
	}
}

// Do выполняет fn под защитой circuit breaker.
// Если автомат разомкнут — возвращает ErrCircuitOpen без вызова fn.
// После успешного fn — сбрасывает счётчик ошибок.
// После ошибки fn — инкрементирует счётчик; при достижении threshold размыкает.
func (b *Breaker) Do(fn func() error) error {
	b.mu.Lock()
	switch b.state {
	case stateOpen:
		if time.Since(b.openedAt) < b.timeout {
			b.mu.Unlock()
			return fmt.Errorf("%w (сервис: %s)", ErrCircuitOpen, b.name)
		}
		// Переходим в Half-Open для пробного запроса.
		b.state = stateHalfOpen
	}
	b.mu.Unlock()

	err := fn()

	b.mu.Lock()
	defer b.mu.Unlock()

	if err != nil {
		b.failures++
		if b.state == stateHalfOpen || b.failures >= b.threshold {
			b.state = stateOpen
			b.openedAt = time.Now()
		}
		return err
	}

	// Успех: сбрасываем в Closed.
	b.failures = 0
	b.state = stateClosed
	return nil
}

// State возвращает текущее состояние для метрик/логирования.
func (b *Breaker) State() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case stateOpen:
		return "open"
	case stateHalfOpen:
		return "half-open"
	default:
		return "closed"
	}
}
