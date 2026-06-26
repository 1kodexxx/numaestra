// Package demolimit реализует защиту расхода кредитов на бесплатные демо поверх
// Redis: глобальный дневной лимит и «одно демо на email» в скользящем окне.
//
// Idempotency: Reserve безопасен к повторам демо-задачи (retry Asynq). Ключ email
// хранит orderID, который его «занял»; повторный вызов с тем же orderID проходит,
// не тратя лимит заново, а другой заказ с тем же email — отклоняется.
package demolimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Limiter — Redis-ограничитель демо.
type Limiter struct {
	rdb         *redis.Client
	dailyLimit  int           // 0 → дневной лимит выключен
	perEmailTTL time.Duration // 0 → лимит на email выключен
}

// New создаёт лимитер. dailyLimit=0 отключает дневной лимит, perEmailHours=0 —
// лимит на email. rdb обязателен.
func New(rdb *redis.Client, dailyLimit, perEmailHours int) *Limiter {
	return &Limiter{
		rdb:         rdb,
		dailyLimit:  dailyLimit,
		perEmailTTL: time.Duration(perEmailHours) * time.Hour,
	}
}

func emailKey(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return "demo:email:" + hex.EncodeToString(sum[:])
}

func dailyKey(now time.Time) string {
	return "demo:count:" + now.UTC().Format("2006-01-02")
}

// Reserve проверяет лимиты и при успехе фиксирует расход для заказа.
//   - email уже получал демо (другим заказом) → (false, nil).
//   - повтор того же заказа → (true, nil), лимит не тратится повторно.
//   - превышен дневной лимит → (false, nil), занятый email-ключ откатывается.
func (l *Limiter) Reserve(ctx context.Context, orderID uuid.UUID, email string) (bool, error) {
	now := time.Now().UTC()

	// 1. Лимит на email (если включён и email задан). Занимаем ключ для этого заказа.
	claimedEmail := false
	var ek string
	if l.perEmailTTL > 0 && strings.TrimSpace(email) != "" {
		ek = emailKey(email)
		ok, err := l.rdb.SetNX(ctx, ek, orderID.String(), l.perEmailTTL).Result()
		if err != nil {
			return false, fmt.Errorf("demolimit: setnx email: %w", err)
		}
		if !ok {
			// Ключ уже есть — наш повтор или чужой заказ.
			owner, err := l.rdb.Get(ctx, ek).Result()
			if err != nil {
				return false, fmt.Errorf("demolimit: get email owner: %w", err)
			}
			if owner == orderID.String() {
				return true, nil // наш retry — уже учтён ранее
			}
			return false, nil // email уже получил демо другим заказом
		}
		claimedEmail = true
	}

	// 2. Дневной лимит (если включён). Инкремент только при свежем заказе.
	if l.dailyLimit > 0 {
		dk := dailyKey(now)
		n, err := l.rdb.Incr(ctx, dk).Result()
		if err != nil {
			l.rollbackEmail(ctx, ek, claimedEmail)
			return false, fmt.Errorf("demolimit: incr daily: %w", err)
		}
		if n == 1 {
			// На сутки + запас, чтобы ключ сам истёк.
			_ = l.rdb.Expire(ctx, dk, 48*time.Hour).Err()
		}
		if int(n) > l.dailyLimit {
			// Превышен лимит — откатываем инкремент и занятый email-ключ.
			_ = l.rdb.Decr(ctx, dk).Err()
			l.rollbackEmail(ctx, ek, claimedEmail)
			return false, nil
		}
	}

	return true, nil
}

// rollbackEmail удаляет только что занятый email-ключ (чтобы заказ не «сжёг» демо
// этого email впустую, когда демо в итоге не стартовало из-за дневного лимита).
func (l *Limiter) rollbackEmail(ctx context.Context, key string, claimed bool) {
	if claimed && key != "" {
		_ = l.rdb.Del(ctx, key).Err()
	}
}
