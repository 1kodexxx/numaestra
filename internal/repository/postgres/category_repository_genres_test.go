package postgres

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v4"

	"github.com/numaestra/numaestra/internal/domain"
)

type stubGenreRepo struct {
	forCategory []domain.Genre
}

func (s *stubGenreRepo) GetAll(context.Context, bool) ([]domain.Genre, error) { return nil, nil }
func (s *stubGenreRepo) GetByID(context.Context, int) (*domain.Genre, error) { return nil, nil }
func (s *stubGenreRepo) GetForCategory(context.Context, string) ([]domain.Genre, error) {
	return s.forCategory, nil
}
func (s *stubGenreRepo) Create(context.Context, *domain.Genre) error   { return nil }
func (s *stubGenreRepo) Update(context.Context, *domain.Genre) error { return nil }
func (s *stubGenreRepo) Delete(context.Context, int) error           { return nil }
func (s *stubGenreRepo) SetCategoryGenres(context.Context, string, []int) error {
	return nil
}
func (s *stubGenreRepo) GetCategoryGenreIDs(context.Context, string) ([]int, error) {
	return nil, nil
}

var _ domain.GenreRepository = (*stubGenreRepo)(nil)

func TestCategoryRepository_GetByID_EnrichesGenreOptions(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	questionsJSON := []byte(`[{
		"id": 5,
		"step_number": 3,
		"question_text": "Жанр",
		"ui_type": "tags",
		"mapping_key": "GENRE",
		"is_required": true,
		"option_source": "genres",
		"config": {"max_select": 3},
		"options": []
	}]`)
	rows := pgxmock.NewRows([]string{"id", "title", "description", "cover_image_url", "seo_tags", "base_prompt_template", "questions"}).
		AddRow("wedding", "Свадьба", "опис", "c.svg", []string{"свадьба"}, "tpl [GENRE]", questionsJSON)
	mock.ExpectQuery("FROM categories c").WithArgs(anyArgs(1)...).WillReturnRows(rows)

	genres := &stubGenreRepo{forCategory: []domain.Genre{
		{ID: 1, Slug: "pop", Label: "Поп", SunoValue: "modern pop", IsActive: true},
		{ID: 2, Slug: "rock", Label: "Рок", SunoValue: "rock", IsActive: true},
	}}
	repo := NewCategoryRepository(mock).WithGenres(genres)
	cat, err := repo.GetByID(context.Background(), "wedding")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	qs := cat.Questions()
	if len(qs) != 1 {
		t.Fatalf("ожидали 1 вопрос, получили %d", len(qs))
	}
	if qs[0].OptionSource != domain.OptionSourceGenres || len(qs[0].Options) != 2 {
		t.Fatalf("жанры не подтянулись: source=%s options=%+v", qs[0].OptionSource, qs[0].Options)
	}
	if qs[0].Options[0].Label != "Поп" || qs[0].Options[0].Value != "modern pop" {
		t.Errorf("неверная опция: %+v", qs[0].Options[0])
	}
}
