package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/numaestra/numaestra/internal/domain"
)

// ExampleUseCase управляет примерами готовых работ: публичный список (главная)
// и админский CRUD. Один usecase обслуживает и публичный, и админский хендлер.
type ExampleUseCase struct {
	repo domain.ExampleRepository
	log  *slog.Logger
}

func NewExampleUseCase(repo domain.ExampleRepository, log *slog.Logger) *ExampleUseCase {
	return &ExampleUseCase{repo: repo, log: log}
}

// ListActive возвращает видимые примеры для публичной главной (по sort_order).
func (uc *ExampleUseCase) ListActive(ctx context.Context) ([]*domain.Example, error) {
	examples, err := uc.repo.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("получение активных примеров: %w", err)
	}
	return examples, nil
}

// List возвращает все примеры (включая скрытые) для админки.
func (uc *ExampleUseCase) List(ctx context.Context) ([]*domain.Example, error) {
	examples, err := uc.repo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("получение списка примеров: %w", err)
	}
	return examples, nil
}

// Get возвращает пример по id.
func (uc *ExampleUseCase) Get(ctx context.Context, id string) (*domain.Example, error) {
	return uc.repo.GetByID(ctx, id)
}

// Create создаёт новый пример.
func (uc *ExampleUseCase) Create(ctx context.Context, id, title, category, description, mood, audioURL, coverURL string, sortOrder int, isActive bool) (*domain.Example, error) {
	e, err := domain.NewExample(id, title, category, description, mood, audioURL, coverURL, sortOrder, isActive)
	if err != nil {
		return nil, fmt.Errorf("валидация примера: %w", err)
	}
	if err := uc.repo.Create(ctx, e); err != nil {
		return nil, err
	}
	uc.log.Info("admin: создан пример", "example_id", id, "title", title)
	return e, nil
}

// Update обновляет изменяемые поля примера.
func (uc *ExampleUseCase) Update(ctx context.Context, id, title, category, description, mood, audioURL, coverURL string, sortOrder int, isActive bool) (*domain.Example, error) {
	e, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := e.UpdateDetails(title, category, description, mood, audioURL, coverURL, sortOrder, isActive); err != nil {
		return nil, fmt.Errorf("валидация примера: %w", err)
	}
	if err := uc.repo.Update(ctx, e); err != nil {
		return nil, err
	}
	uc.log.Info("admin: обновлён пример", "example_id", id)
	return e, nil
}

// Delete удаляет пример.
func (uc *ExampleUseCase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	uc.log.Info("admin: удалён пример", "example_id", id)
	return nil
}
