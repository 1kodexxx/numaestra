package usecase

import (
	"context"
	"strings"

	"github.com/numaestra/numaestra/internal/domain"
)

type PromptUseCase struct {
	categoryRepo domain.CategoryRepository
}

// Добавлен недостающий конструктор
func NewPromptUseCase(categoryRepo domain.CategoryRepository) *PromptUseCase {
	return &PromptUseCase{categoryRepo: categoryRepo}
}

// Обертка для получения всех категорий
func (uc *PromptUseCase) GetAllCategories(ctx context.Context) ([]*domain.Category, error) {
	return uc.categoryRepo.GetAll(ctx)
}

// Обертка для получения конкретной категории с вопросами
func (uc *PromptUseCase) GetCategoryWizard(ctx context.Context, id string) (*domain.Category, error) {
	return uc.categoryRepo.GetByID(ctx, id)
}

// Склейка ответов пользователя с шаблоном
func (uc *PromptUseCase) BuildFinalPrompt(ctx context.Context, categoryID string, userAnswers map[string]string) (string, error) {
	category, err := uc.categoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		return "", err
	}

	finalPrompt := category.BasePromptTemplate

	for key, value := range userAnswers {
		placeholder := "[" + key + "]"
		finalPrompt = strings.ReplaceAll(finalPrompt, placeholder, value)
	}

	return finalPrompt, nil
}
