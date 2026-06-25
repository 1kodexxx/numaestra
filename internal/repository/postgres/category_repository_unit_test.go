package postgres

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v4"

	"github.com/numaestra/numaestra/internal/domain"
)

func TestCategoryRepository_GetAll_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	rows := pgxmock.NewRows([]string{"id", "title", "description", "cover_image_url", "seo_tags"}).
		AddRow("wedding", "Свадьба", "опис", "c.svg", []string{"свадьба", "подарок"}).
		AddRow("corporate", "Корпоратив", "опис2", "c2.svg", []string{})
	mock.ExpectQuery("FROM categories").WillReturnRows(rows)

	repo := NewCategoryRepository(mock)
	cats, err := repo.GetAll(context.Background())
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(cats) != 2 || cats[0].ID() != "wedding" {
		t.Fatalf("неверно разобраны категории: %+v", cats)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestCategoryRepository_GetByID_Success_ParsesQuestions(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	questionsJSON := []byte(`[{"id":1,"step_number":1,"question_text":"Повод?","ui_type":"text","mapping_key":"OCCASION","is_required":true,"options":[]}]`)
	rows := pgxmock.NewRows([]string{"id", "title", "description", "cover_image_url", "seo_tags", "base_prompt_template", "questions"}).
		AddRow("wedding", "Свадьба", "опис", "c.svg", []string{"свадьба"}, "tpl [OCCASION]", questionsJSON)
	mock.ExpectQuery("FROM categories c").WithArgs(anyArgs(1)...).WillReturnRows(rows)

	repo := NewCategoryRepository(mock)
	cat, err := repo.GetByID(context.Background(), "wedding")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if cat.ID() != "wedding" || len(cat.Questions()) != 1 || cat.Questions()[0].MappingKey != "OCCASION" {
		t.Fatalf("неверно разобрана категория/вопросы: %+v", cat.Questions())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestCategoryRepository_Create_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("INSERT INTO categories").WithArgs(anyArgs(6)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))

	repo := NewCategoryRepository(mock)
	cat, _ := domain.NewCategory("wedding", "Свадьба", "", "", nil, "tpl")
	if err := repo.Create(context.Background(), cat); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestCategoryRepository_Update_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE categories").WithArgs(anyArgs(6)...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	repo := NewCategoryRepository(mock)
	cat, _ := domain.NewCategory("wedding", "Свадьба", "", "", nil, "tpl")
	if err := repo.Update(context.Background(), cat); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestCategoryRepository_Delete_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM categories").WithArgs(anyArgs(1)...).WillReturnResult(pgxmock.NewResult("DELETE", 1))

	repo := NewCategoryRepository(mock)
	if err := repo.Delete(context.Background(), "wedding"); err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}

func TestCategoryRepository_AddQuestion_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// INSERT questions ... RETURNING id → QueryRow; затем insertOptions (нет опций — без вставок).
	mock.ExpectQuery("INSERT INTO questions").WithArgs(anyArgs(8)...).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(7))

	repo := NewCategoryRepository(mock)
	q := domain.Question{StepNumber: 1, QuestionText: "Повод?", UIType: "text", MappingKey: "OCCASION", IsRequired: true}
	saved, err := repo.AddQuestion(context.Background(), "wedding", q)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if saved.ID != 7 {
		t.Errorf("ожидали присвоенный id=7, получили %d", saved.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("неудовлетворённые ожидания: %v", err)
	}
}
