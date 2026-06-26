package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
)

type inMemReviewRepo struct {
	items []*domain.Review
}

func (r *inMemReviewRepo) Create(_ context.Context, rev *domain.Review) error {
	r.items = append([]*domain.Review{rev}, r.items...)
	return nil
}
func (r *inMemReviewRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Review, error) {
	for _, it := range r.items {
		if it.ID() == id {
			return it, nil
		}
	}
	return nil, domain.ErrReviewNotFound
}
func (r *inMemReviewRepo) ListPublished(_ context.Context, _, _ int) ([]*domain.Review, error) {
	var out []*domain.Review
	for _, it := range r.items {
		if it.IsPublished() {
			out = append(out, it)
		}
	}
	return out, nil
}
func (r *inMemReviewRepo) CountPublished(_ context.Context) (int, error) {
	n := 0
	for _, it := range r.items {
		if it.IsPublished() {
			n++
		}
	}
	return n, nil
}
func (r *inMemReviewRepo) RatingStats(_ context.Context) (int, float64, error) {
	count, sum := 0, 0
	for _, it := range r.items {
		if it.IsPublished() {
			count++
			sum += it.Rating()
		}
	}
	if count == 0 {
		return 0, 0, nil
	}
	return count, float64(sum) / float64(count), nil
}

func (r *inMemReviewRepo) ListAll(_ context.Context, _, _ int) ([]*domain.Review, error) {
	return r.items, nil
}
func (r *inMemReviewRepo) Update(_ context.Context, rev *domain.Review) error {
	for i, it := range r.items {
		if it.ID() == rev.ID() {
			r.items[i] = rev
			return nil
		}
	}
	return domain.ErrReviewNotFound
}
func (r *inMemReviewRepo) Delete(_ context.Context, id uuid.UUID) error {
	for i, it := range r.items {
		if it.ID() == id {
			r.items = append(r.items[:i], r.items[i+1:]...)
			return nil
		}
	}
	return domain.ErrReviewNotFound
}

var _ domain.ReviewRepository = (*inMemReviewRepo)(nil)

func TestReviewUseCase_CreateAndListPublished(t *testing.T) {
	repo := &inMemReviewRepo{}
	uc := NewReviewUseCase(repo, testLogger())

	if _, err := uc.Create(context.Background(), "Иван", 5, "Отличный сервис"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := uc.Create(context.Background(), "", 5, "x"); err == nil {
		t.Error("пустое имя должно отвергаться")
	}

	reviews, total, err := uc.ListPublished(context.Background(), 0, 0) // page/perPage нормализуются
	if err != nil {
		t.Fatalf("ListPublished: %v", err)
	}
	if total != 1 || len(reviews) != 1 {
		t.Errorf("ожидали 1 опубликованный отзыв, получили total=%d len=%d", total, len(reviews))
	}
}

func TestReviewUseCase_Moderation(t *testing.T) {
	repo := &inMemReviewRepo{}
	uc := NewReviewUseCase(repo, testLogger())
	rev, _ := uc.Create(context.Background(), "Иван", 4, "норм")

	// Ответ.
	if _, err := uc.Reply(context.Background(), rev.ID(), "Спасибо!"); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), rev.ID())
	if got.AdminReply() != "Спасибо!" {
		t.Errorf("ответ не сохранён: %q", got.AdminReply())
	}

	// Скрытие → пропадает из публичного списка, но остаётся в ListAll.
	if _, err := uc.SetPublished(context.Background(), rev.ID(), false); err != nil {
		t.Fatalf("SetPublished: %v", err)
	}
	pub, total, _ := uc.ListPublished(context.Background(), 1, 20)
	if total != 0 || len(pub) != 0 {
		t.Errorf("скрытый отзыв не должен попадать в публичный список")
	}
	all, _ := uc.ListAll(context.Background(), 1, 50)
	if len(all) != 1 {
		t.Errorf("админ должен видеть скрытый отзыв")
	}

	// Удаление.
	if err := uc.Delete(context.Background(), rev.ID()); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := uc.Delete(context.Background(), uuid.New()); err == nil {
		t.Error("удаление несуществующего должно вернуть ошибку")
	}
}

func TestReviewUseCase_RatingStats(t *testing.T) {
	repo := &inMemReviewRepo{}
	uc := NewReviewUseCase(repo, testLogger())
	ctx := context.Background()

	// Нет отзывов — нули.
	count, avg, err := uc.RatingStats(ctx)
	if err != nil {
		t.Fatalf("RatingStats: %v", err)
	}
	if count != 0 || avg != 0 {
		t.Errorf("ожидали 0/0, получили %d/%.2f", count, avg)
	}

	// Добавляем опубликованный отзыв с оценкой 4.
	rev, _ := uc.Create(ctx, "Имя", 4, "Текст")
	_, _ = uc.SetPublished(ctx, rev.ID(), true)

	count, avg, err = uc.RatingStats(ctx)
	if err != nil {
		t.Fatalf("RatingStats после добавления: %v", err)
	}
	if count != 1 {
		t.Errorf("ожидали count=1, получили %d", count)
	}
	if avg != 4.0 {
		t.Errorf("ожидали avg=4.0, получили %.2f", avg)
	}
}
