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

func TestGenreRepository_GetAll_ActiveOnly(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"id", "slug", "label", "suno_value", "sort_order", "is_active"}).
		AddRow(1, "pop", "Поп", "modern pop", 10, true)
	mock.ExpectQuery("FROM genres").WithArgs().WillReturnRows(rows)

	repo := NewGenreRepository(mock)
	genres, err := repo.GetAll(context.Background(), true)
	if err != nil || len(genres) != 1 || genres[0].Slug != "pop" {
		t.Fatalf("GetAll: %+v err=%v", genres, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGenreRepository_Create_Duplicate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("INSERT INTO genres").
		WithArgs(anyArgs(5)...).
		WillReturnError(&pgconn.PgError{Code: pgErrUniqueViolation})

	repo := NewGenreRepository(mock)
	g, _ := domain.NewGenre("pop", "Поп", "modern pop", 10)
	if err := repo.Create(context.Background(), g); !errors.Is(err, domain.ErrGenreAlreadyExists) {
		t.Fatalf("ожидали ErrGenreAlreadyExists, получили %v", err)
	}
}

func TestGenreRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("FROM genres WHERE id").WithArgs(anyArgs(1)...).WillReturnError(pgx.ErrNoRows)

	repo := NewGenreRepository(mock)
	if _, err := repo.GetByID(context.Background(), 99); !errors.Is(err, domain.ErrGenreNotFound) {
		t.Fatalf("ожидали ErrGenreNotFound, получили %v", err)
	}
}

func TestGenreRepository_SetCategoryGenres(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM category_genres").WithArgs(anyArgs(1)...).WillReturnResult(pgxmock.NewResult("DELETE", 2))
	mock.ExpectExec("INSERT INTO category_genres").WithArgs(anyArgs(3)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO category_genres").WithArgs(anyArgs(3)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()

	repo := NewGenreRepository(mock)
	if err := repo.SetCategoryGenres(context.Background(), "wedding", []int{1, 2}); err != nil {
		t.Fatalf("SetCategoryGenres: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
