package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
)

type PromoUseCase struct {
	repo domain.PromoCodeRepository
	log  *slog.Logger
}

func NewPromoUseCase(repo domain.PromoCodeRepository, log *slog.Logger) *PromoUseCase {
	return &PromoUseCase{repo: repo, log: log}
}

// ValidatePromoCode проверяет, что промокод существует и действителен.
func (uc *PromoUseCase) ValidatePromoCode(ctx context.Context, code string) (*domain.PromoCode, error) {
	promo, err := uc.repo.GetByCode(ctx, strings.ToUpper(code))
	if err != nil {
		if errors.Is(err, domain.ErrPromoCodeNotFound) {
			return nil, domain.ErrPromoCodeNotFound
		}
		return nil, fmt.Errorf("получение промокода: %w", err)
	}
	if !promo.IsValid() {
		return nil, domain.ErrPromoCodeInvalid
	}
	return promo, nil
}

// CreatePromoCode создаёт новый промокод (admin).
func (uc *PromoUseCase) CreatePromoCode(ctx context.Context, req CreatePromoRequest) (*domain.PromoCode, error) {
	promo, err := domain.NewPromoCode(req.Code, domain.DiscountType(req.DiscountType), req.DiscountValue, req.Description)
	if err != nil {
		return nil, fmt.Errorf("создание промокода: %w", err)
	}
	if req.MaxUses > 0 {
		n := req.MaxUses
		promo.SetMaxUses(&n)
	}
	if !req.ValidFrom.IsZero() {
		t := req.ValidFrom
		promo.SetValidFrom(&t)
	}
	if !req.ValidUntil.IsZero() {
		t := req.ValidUntil
		promo.SetValidUntil(&t)
	}
	if err := uc.repo.Create(ctx, promo); err != nil {
		return nil, fmt.Errorf("сохранение промокода: %w", err)
	}
	uc.log.Info("создан промокод", "code", promo.Code(), "type", promo.GetDiscountType(), "value", promo.DiscountValue())
	return promo, nil
}

// UpdatePromoCode обновляет параметры промокода (admin).
func (uc *PromoUseCase) UpdatePromoCode(ctx context.Context, id uuid.UUID, req UpdatePromoRequest) (*domain.PromoCode, error) {
	promo, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	promo.SetDescription(req.Description)
	promo.SetActive(req.IsActive)
	if req.MaxUses > 0 {
		n := req.MaxUses
		promo.SetMaxUses(&n)
	} else {
		promo.SetMaxUses(nil)
	}
	if !req.ValidFrom.IsZero() {
		t := req.ValidFrom
		promo.SetValidFrom(&t)
	} else {
		promo.SetValidFrom(nil)
	}
	if !req.ValidUntil.IsZero() {
		t := req.ValidUntil
		promo.SetValidUntil(&t)
	} else {
		promo.SetValidUntil(nil)
	}
	if err := uc.repo.Update(ctx, promo); err != nil {
		return nil, fmt.Errorf("обновление промокода: %w", err)
	}
	return promo, nil
}

func (uc *PromoUseCase) DeletePromoCode(ctx context.Context, id uuid.UUID) error {
	return uc.repo.Delete(ctx, id)
}

func (uc *PromoUseCase) ListPromoCodes(ctx context.Context, limit, offset int) ([]*domain.PromoCode, int, error) {
	promos, err := uc.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := uc.repo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	return promos, total, nil
}

func (uc *PromoUseCase) GetPromoCode(ctx context.Context, id uuid.UUID) (*domain.PromoCode, error) {
	return uc.repo.GetByID(ctx, id)
}

// --- Request DTOs ---

type CreatePromoRequest struct {
	Code          string
	DiscountType  string
	DiscountValue int
	MaxUses       int
	ValidFrom     time.Time
	ValidUntil    time.Time
	Description   string
}

type UpdatePromoRequest struct {
	Description string
	IsActive    bool
	MaxUses     int
	ValidFrom   time.Time
	ValidUntil  time.Time
}
