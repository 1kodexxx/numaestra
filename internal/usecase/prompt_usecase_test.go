package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
)

func TestNewNoopPromptUseCase(t *testing.T) {
	uc := NewNoopPromptUseCase()
	if uc == nil {
		t.Fatal("NewNoopPromptUseCase вернул nil")
	}
}

func TestPromptUseCase_InvalidateCache_Noop(t *testing.T) {
	uc := NewNoopPromptUseCase()
	uc.InvalidateCache()
}

func TestPromptUseCase_GetAllCategories_RepoError(t *testing.T) {
	repo := newInMemCategoryRepo()
	repo.getErr = errors.New("db down")
	uc := NewPromptUseCase(repo)
	_, err := uc.GetAllCategories(context.Background())
	if err == nil {
		t.Fatal("ожидали ошибку при недоступном репозитории")
	}
}

func TestPromptUseCase_GetAllCategories_CacheHit(t *testing.T) {
	repo := newInMemCategoryRepo()
	cat, _ := domain.NewCategory("c1", "Тест", "описание", "", nil, "base_prompt")
	_ = repo.Create(context.Background(), cat)
	uc := NewPromptUseCase(repo)
	ctx := context.Background()

	first, err := uc.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("первый вызов: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("ожидали 1 категорию, получили %d", len(first))
	}
	// Кеш прогрет — репозиторий "падает", но GetAll возвращает из кеша.
	repo.getErr = errors.New("db down")
	second, err := uc.GetAllCategories(ctx)
	if err != nil {
		t.Fatalf("второй вызов (из кеша): %v", err)
	}
	if len(second) != 1 {
		t.Errorf("ожидали 1 категорию из кеша, получили %d", len(second))
	}
}

func TestPromptUseCase_InvalidateCacheClears(t *testing.T) {
	repo := newInMemCategoryRepo()
	cat, _ := domain.NewCategory("c1", "Тест", "описание", "", nil, "base_prompt")
	_ = repo.Create(context.Background(), cat)
	uc := NewPromptUseCase(repo)
	ctx := context.Background()

	_, _ = uc.GetAllCategories(ctx) // прогрев кеша
	uc.InvalidateCache()
	repo.getErr = errors.New("db down")
	_, err := uc.GetAllCategories(ctx)
	if err == nil {
		t.Fatal("после инвалидации кеша должна вернуться ошибка репозитория")
	}
}
