package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/numaestra/numaestra/internal/domain"
)

var reviewCols = []string{"id", "author_name", "rating", "body", "admin_reply", "admin_reply_at", "is_published", "created_at", "updated_at"}

func reviewRow(id uuid.UUID) []any {
	now := time.Now().UTC()
	return []any{id, "Иван", 5, "Отличное приложение", "", (*time.Time)(nil), true, now, now}
}

func mustReview(t *testing.T) *domain.Review {
	t.Helper()
	r, err := domain.NewReview("Иван", 5, "Отличное приложение")
	if err != nil {
		t.Fatalf("NewReview: %v", err)
	}
	return r
}

func TestReviewRepository_Create_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	mock.ExpectExec("INSERT INTO reviews").WithArgs(anyArgs(9)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))

	repo := NewReviewRepository(mock)
	if err := repo.Create(context.Background(), mustReview(t)); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("ожидания: %v", err)
	}
}

func TestReviewRepository_GetByID_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	mock.ExpectQuery("FROM reviews WHERE id =").WithArgs(anyArgs(1)...).WillReturnError(pgx.ErrNoRows)

	repo := NewReviewRepository(mock)
	if _, err := repo.GetByID(context.Background(), uuid.New()); !errors.Is(err, domain.ErrReviewNotFound) {
		t.Fatalf("ожидали ErrReviewNotFound, получили %v", err)
	}
}

func TestReviewRepository_GetByID_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	id := uuid.New()
	mock.ExpectQuery("FROM reviews WHERE id =").WithArgs(anyArgs(1)...).
		WillReturnRows(pgxmock.NewRows(reviewCols).AddRow(reviewRow(id)...))

	repo := NewReviewRepository(mock)
	rev, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if rev.ID() != id || rev.Rating() != 5 || rev.AuthorName() != "Иван" {
		t.Errorf("неверно разобран отзыв: %+v", rev.Snapshot())
	}
}

func TestReviewRepository_ListPublished_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	mock.ExpectQuery("FROM reviews WHERE is_published = TRUE").WithArgs(anyArgs(2)...).
		WillReturnRows(pgxmock.NewRows(reviewCols).AddRow(reviewRow(uuid.New())...).AddRow(reviewRow(uuid.New())...))

	repo := NewReviewRepository(mock)
	list, err := repo.ListPublished(context.Background(), 20, 0)
	if err != nil || len(list) != 2 {
		t.Fatalf("ожидали 2 отзыва, получили %d (%v)", len(list), err)
	}
}

func TestReviewRepository_CountPublished(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	mock.ExpectQuery("COUNT").WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(7))

	repo := NewReviewRepository(mock)
	n, err := repo.CountPublished(context.Background())
	if err != nil || n != 7 {
		t.Fatalf("ожидали 7, получили %d (%v)", n, err)
	}
}

func TestReviewRepository_ListAll_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	mock.ExpectQuery("FROM reviews").WithArgs(anyArgs(2)...).
		WillReturnRows(pgxmock.NewRows(reviewCols).AddRow(reviewRow(uuid.New())...))

	repo := NewReviewRepository(mock)
	list, err := repo.ListAll(context.Background(), 50, 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("ожидали 1 отзыв, получили %d (%v)", len(list), err)
	}
}

func TestReviewRepository_Update_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	mock.ExpectExec("UPDATE reviews").WithArgs(anyArgs(5)...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	repo := NewReviewRepository(mock)
	if err := repo.Update(context.Background(), mustReview(t)); !errors.Is(err, domain.ErrReviewNotFound) {
		t.Fatalf("ожидали ErrReviewNotFound, получили %v", err)
	}
}

func TestReviewRepository_Update_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	mock.ExpectExec("UPDATE reviews").WithArgs(anyArgs(5)...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewReviewRepository(mock)
	if err := repo.Update(context.Background(), mustReview(t)); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
}

func TestReviewRepository_Delete_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	mock.ExpectExec("DELETE FROM reviews").WithArgs(anyArgs(1)...).WillReturnResult(pgxmock.NewResult("DELETE", 0))

	repo := NewReviewRepository(mock)
	if err := repo.Delete(context.Background(), uuid.New()); !errors.Is(err, domain.ErrReviewNotFound) {
		t.Fatalf("ожидали ErrReviewNotFound, получили %v", err)
	}
}

func TestReviewRepository_Delete_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	mock.ExpectExec("DELETE FROM reviews").WithArgs(anyArgs(1)...).WillReturnResult(pgxmock.NewResult("DELETE", 1))

	repo := NewReviewRepository(mock)
	if err := repo.Delete(context.Background(), uuid.New()); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
}
