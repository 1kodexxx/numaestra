//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
)

// setupExamples очищает таблицу examples (миграция её сидит) для изоляции теста.
func setupExamples(t *testing.T) *ExampleRepository {
	t.Helper()
	if testPool == nil {
		t.Skip("пул postgres не инициализирован")
	}
	if _, err := testPool.Exec(context.Background(), `TRUNCATE examples`); err != nil {
		t.Fatalf("очистка examples: %v", err)
	}
	return NewExampleRepository(testPool)
}

func TestIntegration_Example_CRUD_AndActiveFilter(t *testing.T) {
	repo := setupExamples(t)
	ctx := context.Background()

	vis, _ := domain.NewExample("vis", "Видимый", "Свадьба", "опис", "Тепло", "v.mp3", "v.webp", 2, true)
	hid, _ := domain.NewExample("hid", "Скрытый", "Юбилей", "опис2", "Праздник", "h.mp3", "h.webp", 1, false)
	if err := repo.Create(ctx, vis); err != nil {
		t.Fatalf("create vis: %v", err)
	}
	if err := repo.Create(ctx, hid); err != nil {
		t.Fatalf("create hid: %v", err)
	}

	// Дубликат id → ErrExampleAlreadyExists.
	if err := repo.Create(ctx, vis); err != domain.ErrExampleAlreadyExists {
		t.Fatalf("ожидали ErrExampleAlreadyExists, получили %v", err)
	}

	// GetAll: оба, отсортированы по sort_order (hid=1, vis=2).
	all, err := repo.GetAll(ctx)
	if err != nil || len(all) != 2 {
		t.Fatalf("GetAll: ожидали 2, получили %d (%v)", len(all), err)
	}
	if all[0].ID() != "hid" || all[1].ID() != "vis" {
		t.Errorf("неверный порядок по sort_order: %s, %s", all[0].ID(), all[1].ID())
	}

	// GetActive: только видимый.
	active, err := repo.GetActive(ctx)
	if err != nil || len(active) != 1 || active[0].ID() != "vis" {
		t.Fatalf("GetActive: ожидали [vis], получили %+v (%v)", active, err)
	}

	// Update переключает видимость и поля.
	_ = vis.UpdateDetails("Обновлён", "Новая", "опис3", "Драйв", "v2.mp3", "v2.webp", 5, false)
	if err := repo.Update(ctx, vis); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := repo.GetByID(ctx, "vis")
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Title() != "Обновлён" || got.IsActive() {
		t.Errorf("update не применился: %+v", got.Snapshot())
	}

	// Delete + повторное удаление → ErrExampleNotFound.
	if err := repo.Delete(ctx, "vis"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.Delete(ctx, "vis"); err != domain.ErrExampleNotFound {
		t.Fatalf("повторное удаление: ожидали ErrExampleNotFound, получили %v", err)
	}
}

func TestIntegration_OrderStats_Aggregates(t *testing.T) {
	pool := setup(t)
	repo := NewOrderRepository(pool)
	ctx := context.Background()

	// Заказ 1 — оплачен.
	o1 := mustOrder(t, pool)
	if err := o1.MarkPaid(); err != nil {
		t.Fatalf("mark paid: %v", err)
	}
	if err := repo.Update(ctx, o1); err != nil {
		t.Fatalf("update o1: %v", err)
	}
	// Заказ 2 — не оплачен.
	mustOrder(t, pool)

	stats, err := repo.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalOrders != 2 {
		t.Errorf("TotalOrders: ожидали 2, получили %d", stats.TotalOrders)
	}
	if stats.PaidOrders != 1 {
		t.Errorf("PaidOrders: ожидали 1, получили %d", stats.PaidOrders)
	}
	if stats.RevenueKopecks != o1.AmountKopecks() {
		t.Errorf("RevenueKopecks: ожидали %d, получили %d", o1.AmountKopecks(), stats.RevenueKopecks)
	}
	if stats.OrdersToday != 2 {
		t.Errorf("OrdersToday: ожидали 2, получили %d", stats.OrdersToday)
	}
}
