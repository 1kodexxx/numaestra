package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"

	"github.com/numaestra/numaestra/internal/domain"
)

func testCategory(t *testing.T) *domain.Category {
	t.Helper()
	c, err := domain.NewCategory("wedding", "Свадьба", "Песня на свадьбу", "", nil, "Сделай свадебную песню")
	if err != nil {
		t.Fatalf("NewCategory: %v", err)
	}
	return c
}

// Create транслирует SQLSTATE 23505 (unique_violation) в доменную ошибку —
// её невозможно проверить in-memory моком, но можно через pgxmock.
func TestCategoryRepository_Create_Duplicate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("INSERT INTO categories").
		WithArgs(anyArgs(6)...).
		WillReturnError(&pgconn.PgError{Code: pgErrUniqueViolation})

	repo := NewCategoryRepository(mock)
	if err := repo.Create(context.Background(), testCategory(t)); !errors.Is(err, domain.ErrCategoryAlreadyExists) {
		t.Fatalf("ожидали ErrCategoryAlreadyExists, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestCategoryRepository_Update_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE categories").WithArgs(anyArgs(6)...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	repo := NewCategoryRepository(mock)
	if err := repo.Update(context.Background(), testCategory(t)); !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("ожидали ErrCategoryNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestCategoryRepository_Delete_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM categories").WithArgs(anyArgs(1)...).WillReturnResult(pgxmock.NewResult("DELETE", 0))

	repo := NewCategoryRepository(mock)
	if err := repo.Delete(context.Background(), "missing"); !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("ожидали ErrCategoryNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

// AddQuestion к несуществующей категории ловит FK-violation (23503).
func TestCategoryRepository_AddQuestion_CategoryMissing(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO questions").
		WithArgs(anyArgs(6)...).
		WillReturnError(&pgconn.PgError{Code: pgErrForeignKeyViolation})

	repo := NewCategoryRepository(mock)
	_, err = repo.AddQuestion(context.Background(), "missing", domain.Question{StepNumber: 1, QuestionText: "Повод?"})
	if !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("ожидали ErrCategoryNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestCategoryRepository_DeleteQuestion_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM questions").WithArgs(anyArgs(2)...).WillReturnResult(pgxmock.NewResult("DELETE", 0))

	repo := NewCategoryRepository(mock)
	if err := repo.DeleteQuestion(context.Background(), "wedding", 5); !errors.Is(err, domain.ErrQuestionNotFound) {
		t.Fatalf("ожидали ErrQuestionNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestCategoryRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("FROM categories c").WithArgs(anyArgs(1)...).WillReturnError(pgx.ErrNoRows)

	repo := NewCategoryRepository(mock)
	if _, err := repo.GetByID(context.Background(), "missing"); err == nil {
		t.Fatal("ожидали ошибку not found")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}
