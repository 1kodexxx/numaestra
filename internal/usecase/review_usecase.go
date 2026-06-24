package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
)

// ReviewUseCase управляет отзывами о приложении: публичное создание/список и
// админская модерация (ответ, скрытие, удаление). Один usecase на оба хендлера.
type ReviewUseCase struct {
	repo domain.ReviewRepository
	log  *slog.Logger
}

func NewReviewUseCase(repo domain.ReviewRepository, log *slog.Logger) *ReviewUseCase {
	return &ReviewUseCase{repo: repo, log: log}
}

func normPage(page, perPage, defPer, maxPer int) (limit, offset int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > maxPer {
		perPage = defPer
	}
	return perPage, (page - 1) * perPage
}

// Create сохраняет новый публичный отзыв (без регистрации).
func (uc *ReviewUseCase) Create(ctx context.Context, authorName string, rating int, body string) (*domain.Review, error) {
	rev, err := domain.NewReview(authorName, rating, body)
	if err != nil {
		return nil, fmt.Errorf("валидация отзыва: %w", err)
	}
	if err := uc.repo.Create(ctx, rev); err != nil {
		return nil, fmt.Errorf("сохранение отзыва: %w", err)
	}
	uc.log.Info("создан отзыв", "review_id", rev.ID(), "rating", rev.Rating())
	return rev, nil
}

// ListPublished возвращает опубликованные отзывы и их общее число (для публичной страницы).
func (uc *ReviewUseCase) ListPublished(ctx context.Context, page, perPage int) ([]*domain.Review, int, error) {
	limit, offset := normPage(page, perPage, 20, 50)
	total, err := uc.repo.CountPublished(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("подсчёт отзывов: %w", err)
	}
	reviews, err := uc.repo.ListPublished(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("получение отзывов: %w", err)
	}
	return reviews, total, nil
}

// RatingStats возвращает число и среднюю оценку опубликованных отзывов
// (для AggregateRating в JSON-LD на главной).
func (uc *ReviewUseCase) RatingStats(ctx context.Context) (int, float64, error) {
	return uc.repo.RatingStats(ctx)
}

// ListAll возвращает все отзывы (включая скрытые) для админки.
func (uc *ReviewUseCase) ListAll(ctx context.Context, page, perPage int) ([]*domain.Review, error) {
	limit, offset := normPage(page, perPage, 50, 100)
	reviews, err := uc.repo.ListAll(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("получение списка отзывов: %w", err)
	}
	return reviews, nil
}

// Reply задаёт/обновляет ответ администратора на отзыв (пустой — снимает ответ).
func (uc *ReviewUseCase) Reply(ctx context.Context, id uuid.UUID, message string) (*domain.Review, error) {
	rev, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := rev.SetAdminReply(message); err != nil {
		return nil, fmt.Errorf("валидация ответа: %w", err)
	}
	if err := uc.repo.Update(ctx, rev); err != nil {
		return nil, err
	}
	uc.log.Info("admin: ответ на отзыв сохранён", "review_id", id)
	return rev, nil
}

// SetPublished скрывает/показывает отзыв без удаления.
func (uc *ReviewUseCase) SetPublished(ctx context.Context, id uuid.UUID, published bool) (*domain.Review, error) {
	rev, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rev.SetPublished(published)
	if err := uc.repo.Update(ctx, rev); err != nil {
		return nil, err
	}
	uc.log.Info("admin: видимость отзыва изменена", "review_id", id, "published", published)
	return rev, nil
}

// Delete удаляет отзыв.
func (uc *ReviewUseCase) Delete(ctx context.Context, id uuid.UUID) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	uc.log.Info("admin: отзыв удалён", "review_id", id)
	return nil
}
