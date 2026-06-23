package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
)

// inMemExampleRepo — in-memory реализация domain.ExampleRepository для тестов.
type inMemExampleRepo struct {
	items map[string]*domain.Example
}

func newInMemExampleRepo() *inMemExampleRepo {
	return &inMemExampleRepo{items: map[string]*domain.Example{}}
}

func (r *inMemExampleRepo) GetAll(_ context.Context) ([]*domain.Example, error) {
	out := make([]*domain.Example, 0, len(r.items))
	for _, e := range r.items {
		out = append(out, e)
	}
	return out, nil
}

func (r *inMemExampleRepo) GetActive(_ context.Context) ([]*domain.Example, error) {
	var out []*domain.Example
	for _, e := range r.items {
		if e.IsActive() {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *inMemExampleRepo) GetByID(_ context.Context, id string) (*domain.Example, error) {
	e, ok := r.items[id]
	if !ok {
		return nil, domain.ErrExampleNotFound
	}
	return e, nil
}

func (r *inMemExampleRepo) Create(_ context.Context, e *domain.Example) error {
	if _, ok := r.items[e.ID()]; ok {
		return domain.ErrExampleAlreadyExists
	}
	r.items[e.ID()] = e
	return nil
}

func (r *inMemExampleRepo) Update(_ context.Context, e *domain.Example) error {
	if _, ok := r.items[e.ID()]; !ok {
		return domain.ErrExampleNotFound
	}
	r.items[e.ID()] = e
	return nil
}

func (r *inMemExampleRepo) Delete(_ context.Context, id string) error {
	if _, ok := r.items[id]; !ok {
		return domain.ErrExampleNotFound
	}
	delete(r.items, id)
	return nil
}

func TestExampleUseCase_CreateAndList(t *testing.T) {
	uc := NewExampleUseCase(newInMemExampleRepo(), testLogger())
	if _, err := uc.Create(context.Background(), "e1", "Пример", "Свадьба", "", "", "", "", 1, true); err != nil {
		t.Fatalf("Create упал: %v", err)
	}
	list, err := uc.List(context.Background())
	if err != nil || len(list) != 1 {
		t.Fatalf("ожидали 1 пример, получили %d (%v)", len(list), err)
	}
}

func TestExampleUseCase_Create_Duplicate(t *testing.T) {
	uc := NewExampleUseCase(newInMemExampleRepo(), testLogger())
	_, _ = uc.Create(context.Background(), "e1", "A", "", "", "", "", "", 0, true)
	if _, err := uc.Create(context.Background(), "e1", "B", "", "", "", "", "", 0, true); !errors.Is(err, domain.ErrExampleAlreadyExists) {
		t.Fatalf("ожидали ErrExampleAlreadyExists, получили %v", err)
	}
}

func TestExampleUseCase_Update_NotFound(t *testing.T) {
	uc := NewExampleUseCase(newInMemExampleRepo(), testLogger())
	if _, err := uc.Update(context.Background(), "missing", "T", "", "", "", "", "", 0, true); !errors.Is(err, domain.ErrExampleNotFound) {
		t.Fatalf("ожидали ErrExampleNotFound, получили %v", err)
	}
}

func TestExampleUseCase_ListActive_FiltersHidden(t *testing.T) {
	repo := newInMemExampleRepo()
	uc := NewExampleUseCase(repo, testLogger())
	_, _ = uc.Create(context.Background(), "vis", "Видимый", "", "", "", "", "", 1, true)
	_, _ = uc.Create(context.Background(), "hid", "Скрытый", "", "", "", "", "", 2, false)

	active, err := uc.ListActive(context.Background())
	if err != nil {
		t.Fatalf("ListActive упал: %v", err)
	}
	if len(active) != 1 || active[0].ID() != "vis" {
		t.Fatalf("ожидали только видимый пример, получили %+v", active)
	}
}

func TestExampleUseCase_Delete(t *testing.T) {
	uc := NewExampleUseCase(newInMemExampleRepo(), testLogger())
	_, _ = uc.Create(context.Background(), "e1", "A", "", "", "", "", "", 0, true)
	if err := uc.Delete(context.Background(), "e1"); err != nil {
		t.Fatalf("Delete упал: %v", err)
	}
	if err := uc.Delete(context.Background(), "e1"); !errors.Is(err, domain.ErrExampleNotFound) {
		t.Fatalf("повторное удаление должно дать ErrExampleNotFound, получили %v", err)
	}
}

// stubOrderStats — заглушка orderStatsProvider для теста StatsUseCase.
type stubOrderStats struct{ s domain.OrderStats }

func (s stubOrderStats) Stats(_ context.Context) (domain.OrderStats, error) { return s.s, nil }

func TestStatsUseCase_GetStats_Aggregates(t *testing.T) {
	exRepo := newInMemExampleRepo()
	exUC := NewExampleUseCase(exRepo, testLogger())
	_, _ = exUC.Create(context.Background(), "vis", "V", "", "", "", "", "", 1, true)
	_, _ = exUC.Create(context.Background(), "hid", "H", "", "", "", "", "", 2, false)

	catRepo := newInMemCategoryRepo()
	_, _ = NewAdminUseCase(newInMemOrderRepo(), newInMemAccountRepo(), catRepo, &mockRefunder{}, nil, nil, testLogger()).
		CreateCategory(context.Background(), "wedding", "Свадьба", "", "", nil, "tpl")

	orderStats := stubOrderStats{s: domain.OrderStats{TotalOrders: 5, PaidOrders: 3, RevenueKopecks: 600000}}
	uc := NewStatsUseCase(orderStats, newInMemAccountRepo(), catRepo, exRepo, testLogger())

	stats, err := uc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats упал: %v", err)
	}
	if stats.Orders.TotalOrders != 5 || stats.Orders.RevenueKopecks != 600000 {
		t.Errorf("неверные order-агрегаты: %+v", stats.Orders)
	}
	if stats.CategoriesTotal != 1 {
		t.Errorf("ожидали 1 категорию, получили %d", stats.CategoriesTotal)
	}
	if stats.ExamplesTotal != 2 || stats.ExamplesActive != 1 {
		t.Errorf("ожидали 2 примера (1 активный), получили total=%d active=%d", stats.ExamplesTotal, stats.ExamplesActive)
	}
}
