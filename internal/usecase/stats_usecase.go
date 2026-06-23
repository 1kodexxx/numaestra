package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/numaestra/numaestra/internal/domain"
)

// orderStatsProvider — узкий порт агрегатов по заказам. Реализуется postgres
// OrderRepository; вынесен отдельно, чтобы не расширять domain.OrderRepository
// (и не ломать его in-memory моки в тестах).
type orderStatsProvider interface {
	Stats(ctx context.Context) (domain.OrderStats, error)
}

// DashboardStats — сводка для главной страницы админки.
type DashboardStats struct {
	Orders          domain.OrderStats
	AccountsTotal   int
	AccountsActive  int
	TokenBalance    int
	CategoriesTotal int
	ExamplesTotal   int
	ExamplesActive  int
}

// StatsUseCase собирает сводную статистику приложения для дашборда.
type StatsUseCase struct {
	orderStats   orderStatsProvider
	accountRepo  domain.AccountRepository
	categoryRepo domain.CategoryRepository
	exampleRepo  domain.ExampleRepository
	log          *slog.Logger
}

func NewStatsUseCase(
	orderStats orderStatsProvider,
	accountRepo domain.AccountRepository,
	categoryRepo domain.CategoryRepository,
	exampleRepo domain.ExampleRepository,
	log *slog.Logger,
) *StatsUseCase {
	return &StatsUseCase{
		orderStats:   orderStats,
		accountRepo:  accountRepo,
		categoryRepo: categoryRepo,
		exampleRepo:  exampleRepo,
		log:          log,
	}
}

// GetStats собирает дашборд-статистику из заказов, аккаунтов, категорий и примеров.
func (uc *StatsUseCase) GetStats(ctx context.Context) (DashboardStats, error) {
	var stats DashboardStats

	orderStats, err := uc.orderStats.Stats(ctx)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("статистика заказов: %w", err)
	}
	stats.Orders = orderStats

	accounts, err := uc.accountRepo.List(ctx)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("список аккаунтов: %w", err)
	}
	stats.AccountsTotal = len(accounts)
	for _, a := range accounts {
		if a.Status() == domain.AccountStatusActive {
			stats.AccountsActive++
		}
		stats.TokenBalance += a.TokenBalance()
	}

	categories, err := uc.categoryRepo.GetAll(ctx)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("список категорий: %w", err)
	}
	stats.CategoriesTotal = len(categories)

	examples, err := uc.exampleRepo.GetAll(ctx)
	if err != nil {
		return DashboardStats{}, fmt.Errorf("список примеров: %w", err)
	}
	stats.ExamplesTotal = len(examples)
	for _, e := range examples {
		if e.IsActive() {
			stats.ExamplesActive++
		}
	}

	return stats, nil
}
