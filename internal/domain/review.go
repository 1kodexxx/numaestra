package domain

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrReviewNotFound = errors.New("отзыв не найден")

// Ограничения полей отзыва — защита от мусора и переполнения при публичном вводе.
const (
	maxReviewAuthorLen = 80
	maxReviewBodyLen   = 2000
	maxReviewReplyLen  = 2000
)

// ReviewRepository — порт хранилища отзывов о приложении.
type ReviewRepository interface {
	// Create сохраняет новый отзыв.
	Create(ctx context.Context, r *Review) error
	// GetByID возвращает отзыв по id или ErrReviewNotFound.
	GetByID(ctx context.Context, id uuid.UUID) (*Review, error)
	// ListPublished возвращает опубликованные отзывы (публичная страница), новые первыми.
	ListPublished(ctx context.Context, limit, offset int) ([]*Review, error)
	// CountPublished возвращает число опубликованных отзывов (для пагинации/рейтинга).
	CountPublished(ctx context.Context) (int, error)
	// RatingStats возвращает количество и среднюю оценку опубликованных отзывов
	// (для AggregateRating в JSON-LD). При отсутствии отзывов avg = 0.
	RatingStats(ctx context.Context) (count int, avg float64, err error)
	// ListAll возвращает все отзывы (включая скрытые) для админки, новые первыми.
	ListAll(ctx context.Context, limit, offset int) ([]*Review, error)
	// Update перезаписывает изменяемые поля (ответ, публикация); ErrReviewNotFound, если нет.
	Update(ctx context.Context, r *Review) error
	// Delete удаляет отзыв; ErrReviewNotFound, если нет.
	Delete(ctx context.Context, id uuid.UUID) error
}

// Review — агрегат отзыва о приложении. Поля приватные, доступ через геттеры.
type Review struct {
	id           uuid.UUID
	authorName   string
	rating       int
	body         string
	adminReply   string
	adminReplyAt *time.Time
	isPublished  bool
	createdAt    time.Time
	updatedAt    time.Time
}

// NewReview создаёт отзыв после валидации публичного ввода. Публикуется сразу
// (модерация — пост-фактум: администратор может скрыть или удалить).
func NewReview(authorName string, rating int, body string) (*Review, error) {
	authorName = strings.TrimSpace(authorName)
	body = strings.TrimSpace(body)

	if authorName == "" {
		return nil, errors.New("имя обязательно")
	}
	if len([]rune(authorName)) > maxReviewAuthorLen {
		return nil, errors.New("слишком длинное имя")
	}
	if rating < 1 || rating > 5 {
		return nil, errors.New("оценка должна быть от 1 до 5")
	}
	if body == "" {
		return nil, errors.New("текст отзыва обязателен")
	}
	if len([]rune(body)) > maxReviewBodyLen {
		return nil, errors.New("слишком длинный текст отзыва")
	}

	now := time.Now().UTC()
	return &Review{
		id:          uuid.New(),
		authorName:  authorName,
		rating:      rating,
		body:        body,
		isPublished: true,
		createdAt:   now,
		updatedAt:   now,
	}, nil
}

// SetAdminReply задаёт/обновляет ответ администратора. Пустой ответ снимает его.
func (r *Review) SetAdminReply(reply string) error {
	reply = strings.TrimSpace(reply)
	if len([]rune(reply)) > maxReviewReplyLen {
		return errors.New("слишком длинный ответ")
	}
	r.adminReply = reply
	if reply == "" {
		r.adminReplyAt = nil
	} else {
		now := time.Now().UTC()
		r.adminReplyAt = &now
	}
	r.touch()
	return nil
}

// SetPublished скрывает/показывает отзыв без удаления.
func (r *Review) SetPublished(published bool) {
	r.isPublished = published
	r.touch()
}

func (r *Review) touch() { r.updatedAt = time.Now().UTC() }

// ReviewSnapshot — сырые данные для восстановления/сохранения. Только для репозитория.
type ReviewSnapshot struct {
	ID           uuid.UUID
	AuthorName   string
	Rating       int
	Body         string
	AdminReply   string
	AdminReplyAt *time.Time
	IsPublished  bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RestoreReview восстанавливает агрегат из снапшота.
func RestoreReview(s ReviewSnapshot) *Review {
	return &Review{
		id:           s.ID,
		authorName:   s.AuthorName,
		rating:       s.Rating,
		body:         s.Body,
		adminReply:   s.AdminReply,
		adminReplyAt: s.AdminReplyAt,
		isPublished:  s.IsPublished,
		createdAt:    s.CreatedAt,
		updatedAt:    s.UpdatedAt,
	}
}

// Snapshot возвращает снапшот для слоя хранилища.
func (r *Review) Snapshot() ReviewSnapshot {
	return ReviewSnapshot{
		ID:           r.id,
		AuthorName:   r.authorName,
		Rating:       r.rating,
		Body:         r.body,
		AdminReply:   r.adminReply,
		AdminReplyAt: r.adminReplyAt,
		IsPublished:  r.isPublished,
		CreatedAt:    r.createdAt,
		UpdatedAt:    r.updatedAt,
	}
}

// --- Геттеры ---
func (r *Review) ID() uuid.UUID            { return r.id }
func (r *Review) AuthorName() string       { return r.authorName }
func (r *Review) Rating() int              { return r.rating }
func (r *Review) Body() string             { return r.body }
func (r *Review) AdminReply() string       { return r.adminReply }
func (r *Review) AdminReplyAt() *time.Time { return r.adminReplyAt }
func (r *Review) IsPublished() bool        { return r.isPublished }
func (r *Review) CreatedAt() time.Time     { return r.createdAt }
