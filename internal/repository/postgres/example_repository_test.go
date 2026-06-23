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

func testExample(t *testing.T) *domain.Example {
	t.Helper()
	e, err := domain.NewExample("wedding-demo", "Демо свадьба", "Свадьба", "опис", "Тепло", "u.mp3", "c.webp", 1, true)
	if err != nil {
		t.Fatalf("NewExample: %v", err)
	}
	return e
}

func TestExampleRepository_Create_Duplicate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("INSERT INTO examples").
		WithArgs(anyArgs(9)...).
		WillReturnError(&pgconn.PgError{Code: pgErrUniqueViolation})

	repo := NewExampleRepository(mock)
	if err := repo.Create(context.Background(), testExample(t)); !errors.Is(err, domain.ErrExampleAlreadyExists) {
		t.Fatalf("ожидали ErrExampleAlreadyExists, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestExampleRepository_Update_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE examples").WithArgs(anyArgs(9)...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	repo := NewExampleRepository(mock)
	if err := repo.Update(context.Background(), testExample(t)); !errors.Is(err, domain.ErrExampleNotFound) {
		t.Fatalf("ожидали ErrExampleNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestExampleRepository_Delete_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM examples").WithArgs(anyArgs(1)...).WillReturnResult(pgxmock.NewResult("DELETE", 0))

	repo := NewExampleRepository(mock)
	if err := repo.Delete(context.Background(), "missing"); !errors.Is(err, domain.ErrExampleNotFound) {
		t.Fatalf("ожидали ErrExampleNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestExampleRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("FROM examples WHERE id =").WithArgs(anyArgs(1)...).WillReturnError(pgx.ErrNoRows)

	repo := NewExampleRepository(mock)
	if _, err := repo.GetByID(context.Background(), "missing"); !errors.Is(err, domain.ErrExampleNotFound) {
		t.Fatalf("ожидали ErrExampleNotFound, получили %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestExampleRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("INSERT INTO examples").WithArgs(anyArgs(9)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))

	repo := NewExampleRepository(mock)
	if err := repo.Create(context.Background(), testExample(t)); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestExampleRepository_Update_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE examples").WithArgs(anyArgs(9)...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewExampleRepository(mock)
	if err := repo.Update(context.Background(), testExample(t)); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestExampleRepository_GetByID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"id", "title", "category", "description", "mood", "audio_url", "cover_url", "sort_order", "is_active"}).
		AddRow("e1", "Один", "Кат", "д", "м", "u.mp3", "c.webp", 3, true)
	mock.ExpectQuery("FROM examples WHERE id =").WithArgs(anyArgs(1)...).WillReturnRows(rows)

	repo := NewExampleRepository(mock)
	e, err := repo.GetByID(context.Background(), "e1")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if e.ID() != "e1" || e.Title() != "Один" || e.SortOrder() != 3 {
		t.Fatalf("неверно разобран пример: %+v", e.Snapshot())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestExampleRepository_GetAll_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"id", "title", "category", "description", "mood", "audio_url", "cover_url", "sort_order", "is_active"}).
		AddRow("a", "A", "", "", "", "", "", 1, true).
		AddRow("b", "B", "", "", "", "", "", 2, false)
	mock.ExpectQuery("FROM examples").WillReturnRows(rows)

	repo := NewExampleRepository(mock)
	all, err := repo.GetAll(context.Background())
	if err != nil || len(all) != 2 {
		t.Fatalf("ожидали 2 примера, получили %d (%v)", len(all), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestExampleRepository_GetActive_OrdersAndScans(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"id", "title", "category", "description", "mood", "audio_url", "cover_url", "sort_order", "is_active"}).
		AddRow("a", "A", "Cat", "d", "m", "a.mp3", "a.webp", 1, true).
		AddRow("b", "B", "Cat", "d", "m", "b.mp3", "b.webp", 2, true)
	mock.ExpectQuery("FROM examples WHERE is_active = TRUE").WillReturnRows(rows)

	repo := NewExampleRepository(mock)
	examples, err := repo.GetActive(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(examples) != 2 || examples[0].ID() != "a" || !examples[0].IsActive() {
		t.Fatalf("неверно разобраны примеры: %+v", examples)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}
