package apphttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

type genreHandlerRepo struct {
	items []domain.Genre
}

func (r *genreHandlerRepo) GetAll(_ context.Context, activeOnly bool) ([]domain.Genre, error) {
	if !activeOnly {
		return r.items, nil
	}
	var out []domain.Genre
	for _, g := range r.items {
		if g.IsActive {
			out = append(out, g)
		}
	}
	return out, nil
}
func (r *genreHandlerRepo) GetByID(context.Context, int) (*domain.Genre, error) { return nil, nil }
func (r *genreHandlerRepo) GetForCategory(_ context.Context, categoryID string) ([]domain.Genre, error) {
	if categoryID == "wedding" {
		return []domain.Genre{{ID: 1, Slug: "pop", Label: "Поп", SunoValue: "modern pop", IsActive: true}}, nil
	}
	return r.items, nil
}
func (r *genreHandlerRepo) Create(context.Context, *domain.Genre) error { return nil }
func (r *genreHandlerRepo) Update(context.Context, *domain.Genre) error { return nil }
func (r *genreHandlerRepo) Delete(context.Context, int) error           { return nil }
func (r *genreHandlerRepo) SetCategoryGenres(context.Context, string, []int) error {
	return nil
}
func (r *genreHandlerRepo) GetCategoryGenreIDs(context.Context, string) ([]int, error) {
	return nil, nil
}

var _ domain.GenreRepository = (*genreHandlerRepo)(nil)

func TestGenreHandler_List_All(t *testing.T) {
	repo := &genreHandlerRepo{items: []domain.Genre{
		{ID: 1, Slug: "pop", Label: "Поп", SunoValue: "modern pop", IsActive: true},
		{ID: 2, Slug: "old", Label: "Скрытый", SunoValue: "old", IsActive: false},
	}}
	h := NewGenreHandler(usecase.NewGenreUseCase(repo, discardAdminLogger()), discardAdminLogger())
	r := chi.NewRouter()
	r.Mount("/", h.Routes())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", w.Code)
	}
	var resp []domain.Genre
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(resp) != 1 || resp[0].Slug != "pop" {
		t.Errorf("активные жанры: %+v", resp)
	}
}

func TestGenreHandler_List_ForCategory(t *testing.T) {
	repo := &genreHandlerRepo{items: []domain.Genre{
		{ID: 1, Slug: "pop", Label: "Поп", SunoValue: "modern pop", IsActive: true},
		{ID: 2, Slug: "rock", Label: "Рок", SunoValue: "rock", IsActive: true},
	}}
	h := NewGenreHandler(usecase.NewGenreUseCase(repo, discardAdminLogger()), discardAdminLogger())
	r := chi.NewRouter()
	r.Mount("/", h.Routes())

	req := httptest.NewRequest(http.MethodGet, "/?category_id=wedding", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp []domain.Genre
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 1 || resp[0].Slug != "pop" {
		t.Errorf("ожидали жанры категории wedding, получили %+v", resp)
	}
}
