package apphttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

// stubPromptBuilder — минимальный мок usecase.PromptBuilder для unit-тестов.
type stubPromptBuilder struct {
	categories []*domain.Category
	getErr     error
	wizard     *domain.Category
	wizardErr  error
}

func (s *stubPromptBuilder) GetAllCategories(_ context.Context) ([]*domain.Category, error) {
	return s.categories, s.getErr
}

func (s *stubPromptBuilder) GetCategoryWizard(_ context.Context, _ string) (*domain.Category, error) {
	return s.wizard, s.wizardErr
}

func (s *stubPromptBuilder) BuildFinalPrompt(_ context.Context, _ string, _ map[string]string) (string, error) {
	return "", nil
}

var _ usecase.PromptBuilder = (*stubPromptBuilder)(nil)

func categoryTestRouter(h *CategoryHandler) http.Handler {
	r := chi.NewRouter()
	r.Mount("/categories", h.Routes())
	return r
}

func newTestCategoryHandler(pb usecase.PromptBuilder) *CategoryHandler {
	return NewCategoryHandler(pb, discardAdminLogger())
}

func TestCategoryHandler_GetAll_Success(t *testing.T) {
	cats := []*domain.Category{
		domain.RestoreCategory(domain.CategorySnapshot{ID: "wedding", Title: "Свадьба"}),
		domain.RestoreCategory(domain.CategorySnapshot{ID: "bday", Title: "День рождения"}),
	}
	h := newTestCategoryHandler(&stubPromptBuilder{categories: cats})
	router := categoryTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/categories/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("ожидали Content-Type application/json, получили %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc == "" {
		t.Error("ожидали заголовок Cache-Control для списка категорий")
	}

	var resp []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("ошибка декодирования ответа: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("ожидали 2 категории, получили %d", len(resp))
	}
}

func TestCategoryHandler_GetAll_Empty(t *testing.T) {
	h := newTestCategoryHandler(&stubPromptBuilder{categories: []*domain.Category{}})
	router := categoryTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/categories/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", w.Code)
	}
}

func TestCategoryHandler_GetAll_RepoError(t *testing.T) {
	h := newTestCategoryHandler(&stubPromptBuilder{getErr: errors.New("db down")})
	router := categoryTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/categories/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("ожидали 500, получили %d", w.Code)
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp["error"] == nil {
		t.Error("ожидали поле error в теле ответа")
	}
}

func TestCategoryHandler_GetWizard_Found(t *testing.T) {
	wizard := domain.RestoreCategory(domain.CategorySnapshot{
		ID:    "wedding",
		Title: "Свадьба",
		Questions: []domain.Question{
			{ID: 1, MappingKey: "names", QuestionText: "Имена молодожёнов?"},
		},
	})
	h := newTestCategoryHandler(&stubPromptBuilder{wizard: wizard})
	router := categoryTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/categories/wedding/wizard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d; тело: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp["id"] != "wedding" {
		t.Errorf("ожидали id=wedding, получили %v", resp["id"])
	}
}

func TestCategoryHandler_GetWizard_NotFound(t *testing.T) {
	h := newTestCategoryHandler(&stubPromptBuilder{wizardErr: errors.New("not found")})
	router := categoryTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/categories/ghost/wizard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	var resp map[string]any
	json.Unmarshal(body, &resp) //nolint:errcheck
	if resp["error"] == nil {
		t.Error("ожидали поле error в теле ответа")
	}
}
