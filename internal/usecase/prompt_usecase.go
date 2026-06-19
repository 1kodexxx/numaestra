package usecase

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/numaestra/numaestra/internal/domain"
)

const categoriesCacheTTL = time.Hour

type PromptUseCase struct {
	categoryRepo domain.CategoryRepository

	// In-memory TTL-кеш для GetAllCategories. Данные меняются крайне редко
	// (раз в месяц при обновлении каталога), поэтому кешируем на 1 час и не
	// тратим лишний round-trip к Postgres на каждый запрос главной страницы.
	mu         sync.RWMutex
	cachedAll  []*domain.Category
	cachedAt   time.Time
}

// NewPromptUseCase создаёт UseCase для работы с категориями и промптами.
func NewPromptUseCase(categoryRepo domain.CategoryRepository) *PromptUseCase {
	return &PromptUseCase{categoryRepo: categoryRepo}
}

// NewNoopPromptUseCase возвращает UseCase без репозитория — для использования
// в тестах, где категории не задействованы (CreateOrder с пустым categoryID).
func NewNoopPromptUseCase() *PromptUseCase {
	return &PromptUseCase{}
}

// GetAllCategories возвращает список категорий, кешируя результат на categoriesCacheTTL.
// При промахе кеша выполняет запрос к репозиторию и обновляет кеш.
func (uc *PromptUseCase) GetAllCategories(ctx context.Context) ([]*domain.Category, error) {
	uc.mu.RLock()
	if len(uc.cachedAll) > 0 && time.Since(uc.cachedAt) < categoriesCacheTTL {
		cached := uc.cachedAll
		uc.mu.RUnlock()
		return cached, nil
	}
	uc.mu.RUnlock()

	// Промах кеша: загружаем из БД и сохраняем.
	categories, err := uc.categoryRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	uc.mu.Lock()
	uc.cachedAll = categories
	uc.cachedAt = time.Now()
	uc.mu.Unlock()

	return categories, nil
}

// Обертка для получения конкретной категории с вопросами
func (uc *PromptUseCase) GetCategoryWizard(ctx context.Context, id string) (*domain.Category, error) {
	return uc.categoryRepo.GetByID(ctx, id)
}

// BuildFinalPrompt строит итоговый промпт для Suno API, подставляя ответы
// пользователя в шаблон категории. Используется только воркером — не кешируется,
// так как base_prompt_template может меняться независимо от списка категорий.
func (uc *PromptUseCase) BuildFinalPrompt(ctx context.Context, categoryID string, userAnswers map[string]string) (string, error) {
	category, err := uc.categoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		return "", err
	}

	finalPrompt := category.BasePromptTemplate()
	for key, value := range userAnswers {
		placeholder := "[" + key + "]"
		finalPrompt = strings.ReplaceAll(finalPrompt, placeholder, value)
	}

	return finalPrompt, nil
}
