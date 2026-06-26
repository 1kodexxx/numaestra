package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type DiscountType string

const (
	DiscountTypePercent  DiscountType = "percent"
	DiscountTypeFixedRub DiscountType = "fixed_rub"
)

var (
	ErrPromoCodeNotFound = errors.New("промокод не найден")
	ErrPromoCodeInvalid  = errors.New("промокод недействителен или исчерпан")
	ErrPromoCodeInUse    = errors.New("промокод используется в заказах и не может быть удалён")
)

// PromoCode — агрегат промокода со скидкой.
type PromoCode struct {
	id            uuid.UUID
	code          string
	discountType  DiscountType
	discountValue int // процент (1-100) или сумма в рублях
	maxUses       *int
	currentUses   int
	validFrom     *time.Time
	validUntil    *time.Time
	isActive      bool
	description   string
	createdAt     time.Time
}

type PromoCodeSnapshot struct {
	ID            uuid.UUID
	Code          string
	DiscountType  DiscountType
	DiscountValue int
	MaxUses       *int
	CurrentUses   int
	ValidFrom     *time.Time
	ValidUntil    *time.Time
	IsActive      bool
	Description   string
	CreatedAt     time.Time
}

func NewPromoCode(code string, discountType DiscountType, discountValue int, description string) (*PromoCode, error) {
	if code == "" {
		return nil, errors.New("код промокода не может быть пустым")
	}
	if discountType != DiscountTypePercent && discountType != DiscountTypeFixedRub {
		return nil, errors.New("некорректный тип скидки")
	}
	if discountValue <= 0 {
		return nil, errors.New("значение скидки должно быть положительным")
	}
	if discountType == DiscountTypePercent && discountValue > 100 {
		return nil, errors.New("скидка в процентах не может превышать 100")
	}
	return &PromoCode{
		id: uuid.New(), code: code, discountType: discountType,
		discountValue: discountValue, isActive: true,
		description: description, createdAt: time.Now().UTC(),
	}, nil
}

func RestorePromoCode(s PromoCodeSnapshot) *PromoCode {
	return &PromoCode{
		id: s.ID, code: s.Code, discountType: s.DiscountType,
		discountValue: s.DiscountValue, maxUses: s.MaxUses, currentUses: s.CurrentUses,
		validFrom: s.ValidFrom, validUntil: s.ValidUntil, isActive: s.IsActive,
		description: s.Description, createdAt: s.CreatedAt,
	}
}

// --- Геттеры ---

func (p *PromoCode) ID() uuid.UUID                 { return p.id }
func (p *PromoCode) Code() string                  { return p.code }
func (p *PromoCode) GetDiscountType() DiscountType { return p.discountType }
func (p *PromoCode) DiscountValue() int            { return p.discountValue }
func (p *PromoCode) MaxUses() *int                 { return p.maxUses }
func (p *PromoCode) CurrentUses() int              { return p.currentUses }
func (p *PromoCode) ValidFrom() *time.Time         { return p.validFrom }
func (p *PromoCode) ValidUntil() *time.Time        { return p.validUntil }
func (p *PromoCode) Active() bool                  { return p.isActive }
func (p *PromoCode) Description() string           { return p.description }
func (p *PromoCode) CreatedAt() time.Time          { return p.createdAt }

// IsValid проверяет, что промокод можно применить прямо сейчас.
func (p *PromoCode) IsValid() bool {
	if !p.isActive {
		return false
	}
	if p.maxUses != nil && p.currentUses >= *p.maxUses {
		return false
	}
	now := time.Now().UTC()
	if p.validFrom != nil && now.Before(*p.validFrom) {
		return false
	}
	if p.validUntil != nil && now.After(*p.validUntil) {
		return false
	}
	return true
}

// Apply вычисляет сумму скидки в копейках.
func (p *PromoCode) Apply(amountKopecks int64) int64 {
	var d int64
	switch p.discountType {
	case DiscountTypePercent:
		d = amountKopecks * int64(p.discountValue) / 100
	case DiscountTypeFixedRub:
		d = int64(p.discountValue) * 100 // рубли → копейки
	}
	if d > amountKopecks {
		d = amountKopecks
	}
	return d
}

func (p *PromoCode) SetMaxUses(n *int)          { p.maxUses = n }
func (p *PromoCode) SetValidFrom(t *time.Time)  { p.validFrom = t }
func (p *PromoCode) SetValidUntil(t *time.Time) { p.validUntil = t }
func (p *PromoCode) SetActive(v bool)           { p.isActive = v }
func (p *PromoCode) SetDescription(s string)    { p.description = s }

func (p *PromoCode) Snapshot() PromoCodeSnapshot {
	return PromoCodeSnapshot{
		ID: p.id, Code: p.code, DiscountType: p.discountType,
		DiscountValue: p.discountValue, MaxUses: p.maxUses, CurrentUses: p.currentUses,
		ValidFrom: p.validFrom, ValidUntil: p.validUntil, IsActive: p.isActive,
		Description: p.description, CreatedAt: p.createdAt,
	}
}

// PromoCodeRepository — контракт для хранения промокодов.
type PromoCodeRepository interface {
	GetByCode(ctx context.Context, code string) (*PromoCode, error)
	GetByID(ctx context.Context, id uuid.UUID) (*PromoCode, error)
	Create(ctx context.Context, promo *PromoCode) error
	Update(ctx context.Context, promo *PromoCode) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*PromoCode, error)
	Count(ctx context.Context) (int, error)
	// IncrementUses атомарно увеличивает счётчик использований, уважая max_uses.
	// Возвращает false, если лимит уже исчерпан (оптимистичная конкуренция).
	IncrementUses(ctx context.Context, id uuid.UUID) (ok bool, err error)
}
