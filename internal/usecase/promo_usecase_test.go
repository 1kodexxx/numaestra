package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
)

// --- in-memory прокси репозиторий ---

type inMemPromoRepo struct {
	byCode  map[string]*domain.PromoCode
	byID    map[uuid.UUID]*domain.PromoCode
	listErr error
}

func newInMemPromoRepo() *inMemPromoRepo {
	return &inMemPromoRepo{
		byCode: make(map[string]*domain.PromoCode),
		byID:   make(map[uuid.UUID]*domain.PromoCode),
	}
}

func (r *inMemPromoRepo) GetByCode(_ context.Context, code string) (*domain.PromoCode, error) {
	p, ok := r.byCode[code]
	if !ok {
		return nil, domain.ErrPromoCodeNotFound
	}
	return p, nil
}

func (r *inMemPromoRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.PromoCode, error) {
	p, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrPromoCodeNotFound
	}
	return p, nil
}

func (r *inMemPromoRepo) Create(_ context.Context, p *domain.PromoCode) error {
	r.byCode[p.Code()] = p
	r.byID[p.ID()] = p
	return nil
}

func (r *inMemPromoRepo) Update(_ context.Context, p *domain.PromoCode) error {
	r.byCode[p.Code()] = p
	r.byID[p.ID()] = p
	return nil
}

func (r *inMemPromoRepo) Delete(_ context.Context, id uuid.UUID) error {
	p, ok := r.byID[id]
	if !ok {
		return domain.ErrPromoCodeNotFound
	}
	delete(r.byCode, p.Code())
	delete(r.byID, id)
	return nil
}

func (r *inMemPromoRepo) List(_ context.Context, limit, _ int) ([]*domain.PromoCode, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	out := make([]*domain.PromoCode, 0, len(r.byID))
	for _, p := range r.byID {
		out = append(out, p)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *inMemPromoRepo) Count(_ context.Context) (int, error) {
	if r.listErr != nil {
		return 0, r.listErr
	}
	return len(r.byID), nil
}

func (r *inMemPromoRepo) IncrementUses(_ context.Context, id uuid.UUID) (bool, error) {
	p, ok := r.byID[id]
	if !ok {
		return false, domain.ErrPromoCodeNotFound
	}
	snap := p.Snapshot()
	if snap.MaxUses != nil && snap.CurrentUses >= *snap.MaxUses {
		return false, nil
	}
	snap.CurrentUses++
	restored := domain.RestorePromoCode(snap)
	r.byID[id] = restored
	r.byCode[snap.Code] = restored
	return true, nil
}

// --- helpers ---

func newPromoFixture() (*PromoUseCase, *inMemPromoRepo) {
	repo := newInMemPromoRepo()
	uc := NewPromoUseCase(repo, testLogger())
	return uc, repo
}

// --- тесты ---

func TestPromoUseCase_CreateAndValidate(t *testing.T) {
	uc, _ := newPromoFixture()
	ctx := context.Background()

	_, err := uc.CreatePromoCode(ctx, CreatePromoRequest{
		Code: "TEST10", DiscountType: "percent", DiscountValue: 10,
	})
	if err != nil {
		t.Fatalf("CreatePromoCode: %v", err)
	}

	promo, err := uc.ValidatePromoCode(ctx, "test10") // lowercase → uppercase внутри
	if err != nil {
		t.Fatalf("ValidatePromoCode: %v", err)
	}
	if promo.Code() != "TEST10" {
		t.Errorf("неверный код: %q", promo.Code())
	}
}

func TestPromoUseCase_ValidateNotFound(t *testing.T) {
	uc, _ := newPromoFixture()
	_, err := uc.ValidatePromoCode(context.Background(), "MISSING")
	if !errors.Is(err, domain.ErrPromoCodeNotFound) {
		t.Fatalf("ожидали ErrPromoCodeNotFound, получили %v", err)
	}
}

func TestPromoUseCase_ValidateInactive(t *testing.T) {
	uc, _ := newPromoFixture()
	ctx := context.Background()

	p, _ := uc.CreatePromoCode(ctx, CreatePromoRequest{
		Code: "OFF", DiscountType: "percent", DiscountValue: 5,
	})
	_, _ = uc.UpdatePromoCode(ctx, p.ID(), UpdatePromoRequest{IsActive: false})

	_, err := uc.ValidatePromoCode(ctx, "OFF")
	if !errors.Is(err, domain.ErrPromoCodeInvalid) {
		t.Fatalf("ожидали ErrPromoCodeInvalid, получили %v", err)
	}
}

func TestPromoUseCase_UpdateAndGet(t *testing.T) {
	uc, _ := newPromoFixture()
	ctx := context.Background()

	p, _ := uc.CreatePromoCode(ctx, CreatePromoRequest{
		Code: "UPDT", DiscountType: "fixed_rub", DiscountValue: 200, MaxUses: 5,
	})

	updated, err := uc.UpdatePromoCode(ctx, p.ID(), UpdatePromoRequest{
		Description: "обновлено", IsActive: true, MaxUses: intPtr(10),
	})
	if err != nil {
		t.Fatalf("UpdatePromoCode: %v", err)
	}
	if updated.Description() != "обновлено" {
		t.Errorf("неверное описание: %q", updated.Description())
	}
	if updated.MaxUses() == nil || *updated.MaxUses() != 10 {
		t.Errorf("неверный MaxUses: %v", updated.MaxUses())
	}

	got, err := uc.GetPromoCode(ctx, p.ID())
	if err != nil {
		t.Fatalf("GetPromoCode: %v", err)
	}
	if got.ID() != p.ID() {
		t.Error("неверный ID при GetPromoCode")
	}
}

func TestPromoUseCase_UpdateRemovesMaxUses(t *testing.T) {
	uc, _ := newPromoFixture()
	ctx := context.Background()

	p, _ := uc.CreatePromoCode(ctx, CreatePromoRequest{
		Code: "RM", DiscountType: "percent", DiscountValue: 5, MaxUses: 3,
	})
	updated, _ := uc.UpdatePromoCode(ctx, p.ID(), UpdatePromoRequest{
		IsActive: true, MaxUses: intPtr(0), // ptr(0) → снять лимит
	})
	if updated.MaxUses() != nil {
		t.Errorf("ожидали nil MaxUses, получили %v", updated.MaxUses())
	}
}

func TestPromoUseCase_Delete(t *testing.T) {
	uc, _ := newPromoFixture()
	ctx := context.Background()

	p, _ := uc.CreatePromoCode(ctx, CreatePromoRequest{
		Code: "DEL", DiscountType: "percent", DiscountValue: 5,
	})
	if err := uc.DeletePromoCode(ctx, p.ID()); err != nil {
		t.Fatalf("DeletePromoCode: %v", err)
	}

	_, err := uc.GetPromoCode(ctx, p.ID())
	if !errors.Is(err, domain.ErrPromoCodeNotFound) {
		t.Errorf("ожидали ErrPromoCodeNotFound после удаления, получили %v", err)
	}
}

func TestPromoUseCase_ListAndCount(t *testing.T) {
	uc, _ := newPromoFixture()
	ctx := context.Background()

	for _, code := range []string{"A1", "B2", "C3"} {
		_, err := uc.CreatePromoCode(ctx, CreatePromoRequest{
			Code: code, DiscountType: "percent", DiscountValue: 5,
		})
		if err != nil {
			t.Fatalf("CreatePromoCode %q: %v", code, err)
		}
	}

	promos, total, err := uc.ListPromoCodes(ctx, 20, 0)
	if err != nil {
		t.Fatalf("ListPromoCodes: %v", err)
	}
	if len(promos) != 3 {
		t.Errorf("ожидали 3 промокода, получили %d", len(promos))
	}
	if total != 3 {
		t.Errorf("ожидали total=3, получили %d", total)
	}
}

func TestPromoUseCase_CreateInvalidCode(t *testing.T) {
	uc, _ := newPromoFixture()
	_, err := uc.CreatePromoCode(context.Background(), CreatePromoRequest{
		Code: "", DiscountType: "percent", DiscountValue: 10,
	})
	if err == nil {
		t.Fatal("ожидали ошибку для пустого кода")
	}
}

func TestPromoUseCase_UpdateNotFound(t *testing.T) {
	uc, _ := newPromoFixture()
	_, err := uc.UpdatePromoCode(context.Background(), uuid.New(), UpdatePromoRequest{IsActive: true})
	if !errors.Is(err, domain.ErrPromoCodeNotFound) {
		t.Fatalf("ожидали ErrPromoCodeNotFound, получили %v", err)
	}
}
