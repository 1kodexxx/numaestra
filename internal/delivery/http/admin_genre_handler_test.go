package apphttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

type adminGenreRepo struct {
	mu             sync.Mutex
	genres         map[int]domain.Genre
	nextID         int
	categoryGenres map[string][]int
}

func newAdminGenreRepo() *adminGenreRepo {
	return &adminGenreRepo{
		genres:         make(map[int]domain.Genre),
		categoryGenres: make(map[string][]int),
		nextID:         1,
	}
}

func (r *adminGenreRepo) GetAll(_ context.Context, _ bool) ([]domain.Genre, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Genre, 0, len(r.genres))
	for _, g := range r.genres {
		out = append(out, g)
	}
	return out, nil
}

func (r *adminGenreRepo) GetByID(_ context.Context, id int) (*domain.Genre, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.genres[id]
	if !ok {
		return nil, domain.ErrGenreNotFound
	}
	cp := g
	return &cp, nil
}

func (r *adminGenreRepo) GetForCategory(context.Context, string) ([]domain.Genre, error) {
	return nil, nil
}

func (r *adminGenreRepo) Create(_ context.Context, g *domain.Genre) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.genres {
		if x.Slug == g.Slug {
			return domain.ErrGenreAlreadyExists
		}
	}
	g.ID = r.nextID
	r.nextID++
	r.genres[g.ID] = *g
	return nil
}

func (r *adminGenreRepo) Update(_ context.Context, g *domain.Genre) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.genres[g.ID]; !ok {
		return domain.ErrGenreNotFound
	}
	r.genres[g.ID] = *g
	return nil
}

func (r *adminGenreRepo) Delete(_ context.Context, id int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.genres[id]; !ok {
		return domain.ErrGenreNotFound
	}
	delete(r.genres, id)
	return nil
}

func (r *adminGenreRepo) SetCategoryGenres(_ context.Context, categoryID string, genreIDs []int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.categoryGenres[categoryID] = append([]int(nil), genreIDs...)
	return nil
}

func (r *adminGenreRepo) GetCategoryGenreIDs(_ context.Context, categoryID string) ([]int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int(nil), r.categoryGenres[categoryID]...), nil
}

var _ domain.GenreRepository = (*adminGenreRepo)(nil)

func newTestAdminHandlerWithGenres(t *testing.T) (*AdminHandler, *adminGenreRepo) {
	t.Helper()
	genres := newAdminGenreRepo()
	uc := usecase.NewAdminUseCase(newAdminOrderRepo(), newAdminAccRepo(), newAdminCategoryRepo(), &noopRefunder{}, nil, nil, discardAdminLogger())
	genreUC := usecase.NewGenreUseCase(genres, discardAdminLogger())
	return NewAdminHandler(uc, discardAdminLogger()).WithGenres(genreUC), genres
}

func TestAdminHandler_ListGenres_Success(t *testing.T) {
	h, repo := newTestAdminHandlerWithGenres(t)
	router := adminTestRouter(h)
	g, _ := domain.NewGenre("pop", "Поп", "modern pop", 10)
	_ = repo.Create(context.Background(), g)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/genres/", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", w.Code, w.Body.String())
	}
	var resp []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp) != 1 || resp[0]["slug"] != "pop" {
		t.Errorf("ожидали 1 жанр pop, получили %+v", resp)
	}
}

func TestAdminHandler_CreateGenre_Success(t *testing.T) {
	h, repo := newTestAdminHandlerWithGenres(t)
	router := adminTestRouter(h)

	body := `{"slug":"rock","label":"Рок","suno_value":"rock","sort_order":20}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/genres/", strings.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d (%s)", w.Code, w.Body.String())
	}
	if len(repo.genres) != 1 {
		t.Errorf("жанр должен быть в репозитории, получили %d", len(repo.genres))
	}
}

func TestAdminHandler_CreateGenre_Duplicate(t *testing.T) {
	h, repo := newTestAdminHandlerWithGenres(t)
	router := adminTestRouter(h)
	g, _ := domain.NewGenre("pop", "Поп", "modern pop", 10)
	_ = repo.Create(context.Background(), g)

	body := `{"slug":"pop","label":"Поп 2","suno_value":"pop","sort_order":10}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/genres/", strings.NewReader(body)))
	if w.Code != http.StatusConflict {
		t.Fatalf("ожидали 409, получили %d", w.Code)
	}
}

func TestAdminHandler_SetCategoryGenres_Success(t *testing.T) {
	h, repo := newTestAdminHandlerWithGenres(t)
	router := adminTestRouter(h)
	g, _ := domain.NewGenre("pop", "Поп", "modern pop", 10)
	_ = repo.Create(context.Background(), g)

	body := `{"genre_ids":[1]}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPut, "/admin/categories/wedding/genres", strings.NewReader(body)))
	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d (%s)", w.Code, w.Body.String())
	}
	ids, _ := repo.GetCategoryGenreIDs(context.Background(), "wedding")
	if len(ids) != 1 || ids[0] != 1 {
		t.Errorf("ожидали genre_ids [1], получили %v", ids)
	}
}

func TestAdminHandler_AddQuestion_GenresSource(t *testing.T) {
	h, _ := newTestAdminHandlerWithGenres(t)
	router := adminTestRouter(h)

	createBody := `{"id":"wedding","title":"Свадьба","base_prompt_template":"шаблон"}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(createBody)))

	qBody := `{
		"step_number": 3,
		"question_text": "Жанр",
		"ui_type": "tags",
		"mapping_key": "GENRE",
		"is_required": true,
		"option_source": "genres",
		"config": {"max_select": 3},
		"options": []
	}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/categories/wedding/questions", strings.NewReader(qBody)))
	if w.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d (%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["option_source"] != "genres" {
		t.Errorf("ожидали option_source=genres, получили %+v", resp)
	}
}
