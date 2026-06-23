package apphttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

// --- in-memory CategoryRepository для HTTP-тестов admin-эндпоинтов категорий ---

type adminCategoryRepo struct {
	mu         sync.Mutex
	categories map[string]domain.CategorySnapshot
}

func newAdminCategoryRepo() *adminCategoryRepo {
	return &adminCategoryRepo{categories: make(map[string]domain.CategorySnapshot)}
}

func (r *adminCategoryRepo) GetAll(_ context.Context) ([]*domain.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domain.Category, 0, len(r.categories))
	for _, s := range r.categories {
		out = append(out, domain.RestoreCategory(s))
	}
	return out, nil
}

func (r *adminCategoryRepo) GetByID(_ context.Context, id string) (*domain.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.categories[id]
	if !ok {
		return nil, domain.ErrCategoryNotFound
	}
	return domain.RestoreCategory(s), nil
}

func (r *adminCategoryRepo) Create(_ context.Context, c *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := c.Snapshot()
	if _, exists := r.categories[snap.ID]; exists {
		return domain.ErrCategoryAlreadyExists
	}
	r.categories[snap.ID] = snap
	return nil
}

func (r *adminCategoryRepo) Update(_ context.Context, c *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := c.Snapshot()
	if _, exists := r.categories[snap.ID]; !exists {
		return domain.ErrCategoryNotFound
	}
	r.categories[snap.ID] = snap
	return nil
}

func (r *adminCategoryRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.categories[id]; !exists {
		return domain.ErrCategoryNotFound
	}
	delete(r.categories, id)
	return nil
}

func (r *adminCategoryRepo) AddQuestion(_ context.Context, categoryID string, q domain.Question) (domain.Question, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.categories[categoryID]
	if !ok {
		return domain.Question{}, domain.ErrCategoryNotFound
	}
	q.ID = len(snap.Questions) + 1
	snap.Questions = append(snap.Questions, q)
	r.categories[categoryID] = snap
	return q, nil
}

func (r *adminCategoryRepo) UpdateQuestion(_ context.Context, categoryID string, q domain.Question) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.categories[categoryID]
	if !ok {
		return domain.ErrQuestionNotFound
	}
	for i, existing := range snap.Questions {
		if existing.ID == q.ID {
			snap.Questions[i] = q
			r.categories[categoryID] = snap
			return nil
		}
	}
	return domain.ErrQuestionNotFound
}

func (r *adminCategoryRepo) DeleteQuestion(_ context.Context, categoryID string, questionID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap, ok := r.categories[categoryID]
	if !ok {
		return domain.ErrQuestionNotFound
	}
	for i, existing := range snap.Questions {
		if existing.ID == questionID {
			snap.Questions = append(snap.Questions[:i], snap.Questions[i+1:]...)
			r.categories[categoryID] = snap
			return nil
		}
	}
	return domain.ErrQuestionNotFound
}

var _ domain.CategoryRepository = (*adminCategoryRepo)(nil)

func newTestAdminHandlerWithCategories(t *testing.T) (*AdminHandler, *adminCategoryRepo) {
	t.Helper()
	categories := newAdminCategoryRepo()
	uc := usecase.NewAdminUseCase(newAdminOrderRepo(), newAdminAccRepo(), categories, &noopRefunder{}, nil, nil, discardAdminLogger())
	return NewAdminHandler(uc, discardAdminLogger()), categories
}

// --- ListCategories / GetCategory ---

func TestAdminHandler_ListCategories_Success(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	createBody := `{"id":"wedding","title":"Свадьба","base_prompt_template":"шаблон"}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(createBody)))

	r := httptest.NewRequest(http.MethodGet, "/admin/categories/", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", w.Code, w.Body.String())
	}
	var resp []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("не удалось разобрать ответ: %v", err)
	}
	if len(resp) != 1 || resp[0]["id"] != "wedding" {
		t.Errorf("ожидали 1 категорию wedding, получили %+v", resp)
	}
	if resp[0]["base_prompt_template"] != "шаблон" {
		t.Error("admin-ответ должен включать base_prompt_template (в отличие от публичного API)")
	}
}

func TestAdminHandler_GetCategory_Success(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	createBody := `{"id":"wedding","title":"Свадьба","base_prompt_template":"шаблон"}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(createBody)))

	r := httptest.NewRequest(http.MethodGet, "/admin/categories/wedding", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", w.Code, w.Body.String())
	}
}

func TestAdminHandler_GetCategory_NotFound(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/admin/categories/несуществующая", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
}

// --- CreateCategory ---

func TestAdminHandler_CreateCategory_Success(t *testing.T) {
	h, categories := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	body := `{"id":"general","title":"Свободная тема","description":"Опишите песню своими словами","seo_tags":["своя тема"],"base_prompt_template":"Create a song about: [BRIEF]"}`
	r := httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d (%s)", w.Code, w.Body.String())
	}
	if _, ok := categories.categories["general"]; !ok {
		t.Error("категория должна быть сохранена")
	}
}

func TestAdminHandler_CreateCategory_DuplicateID(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	body := `{"id":"wedding","title":"Свадьба","base_prompt_template":"шаблон"}`
	r1 := httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(body))
	router.ServeHTTP(httptest.NewRecorder(), r1)

	r2 := httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(body))
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, r2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("ожидали 409 при дублирующемся id, получили %d", w2.Code)
	}
}

func TestAdminHandler_CreateCategory_ValidationError(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	body := `{"id":"","title":"Заголовок","base_prompt_template":"шаблон"}`
	r := httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 при пустом id, получили %d", w.Code)
	}
}

// --- UpdateCategory ---

func TestAdminHandler_UpdateCategory_Success(t *testing.T) {
	h, categories := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	createBody := `{"id":"wedding","title":"Свадьба","base_prompt_template":"шаблон"}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(createBody)))

	updateBody := `{"title":"Свадьба (обновлено)","description":"новое описание","base_prompt_template":"новый шаблон"}`
	r := httptest.NewRequest(http.MethodPut, "/admin/categories/wedding", strings.NewReader(updateBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", w.Code, w.Body.String())
	}
	if categories.categories["wedding"].Title != "Свадьба (обновлено)" {
		t.Errorf("title не обновился: %q", categories.categories["wedding"].Title)
	}
}

func TestAdminHandler_UpdateCategory_NotFound(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	body := `{"title":"Заголовок","base_prompt_template":"шаблон"}`
	r := httptest.NewRequest(http.MethodPut, "/admin/categories/несуществующая", strings.NewReader(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
}

// --- DeleteCategory ---

func TestAdminHandler_DeleteCategory_Success(t *testing.T) {
	h, categories := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	createBody := `{"id":"boss","title":"Босс","base_prompt_template":"шаблон"}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(createBody)))

	r := httptest.NewRequest(http.MethodDelete, "/admin/categories/boss", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d", w.Code)
	}
	if _, ok := categories.categories["boss"]; ok {
		t.Error("категория должна быть удалена")
	}
}

func TestAdminHandler_DeleteCategory_NotFound(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodDelete, "/admin/categories/несуществующая", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
}

// --- Questions ---

func TestAdminHandler_AddQuestion_Success(t *testing.T) {
	h, categories := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	createBody := `{"id":"wedding","title":"Свадьба","base_prompt_template":"шаблон"}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(createBody)))

	qBody := `{"step_number":1,"question_text":"Как зовут жениха?","ui_type":"text","mapping_key":"GROOM","is_required":true}`
	r := httptest.NewRequest(http.MethodPost, "/admin/categories/wedding/questions", strings.NewReader(qBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d (%s)", w.Code, w.Body.String())
	}
	if len(categories.categories["wedding"].Questions) != 1 {
		t.Errorf("ожидали 1 вопрос, получили %d", len(categories.categories["wedding"].Questions))
	}
}

func TestAdminHandler_AddQuestion_CategoryNotFound(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	qBody := `{"step_number":1,"question_text":"Текст?","ui_type":"text","mapping_key":"KEY","is_required":true}`
	r := httptest.NewRequest(http.MethodPost, "/admin/categories/несуществующая/questions", strings.NewReader(qBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
}

func TestAdminHandler_AddQuestion_ValidationError(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	createBody := `{"id":"wedding","title":"Свадьба","base_prompt_template":"шаблон"}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(createBody)))

	// ui_type "tags" без options — должно быть отклонено валидацией домена.
	qBody := `{"step_number":1,"question_text":"Жанр?","ui_type":"tags","mapping_key":"GENRE","is_required":true}`
	r := httptest.NewRequest(http.MethodPost, "/admin/categories/wedding/questions", strings.NewReader(qBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400, получили %d", w.Code)
	}
}

func TestAdminHandler_UpdateQuestion_Success(t *testing.T) {
	h, categories := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	createBody := `{"id":"wedding","title":"Свадьба","base_prompt_template":"шаблон"}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(createBody)))

	addBody := `{"step_number":1,"question_text":"Как зовут жениха?","ui_type":"text","mapping_key":"GROOM","is_required":true}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/wedding/questions", strings.NewReader(addBody)))

	qid := categories.categories["wedding"].Questions[0].ID
	updateBody := `{"step_number":2,"question_text":"Имя жениха (обновлено)?","ui_type":"text","mapping_key":"GROOM","is_required":false}`
	r := httptest.NewRequest(http.MethodPut, "/admin/categories/wedding/questions/"+strconv.Itoa(qid), strings.NewReader(updateBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d (%s)", w.Code, w.Body.String())
	}
	if categories.categories["wedding"].Questions[0].QuestionText != "Имя жениха (обновлено)?" {
		t.Errorf("вопрос не обновился: %+v", categories.categories["wedding"].Questions[0])
	}
}

func TestAdminHandler_UpdateQuestion_InvalidID(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	r := httptest.NewRequest(http.MethodPut, "/admin/categories/wedding/questions/not-a-number", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("ожидали 400 для некорректного id вопроса, получили %d", w.Code)
	}
}

func TestAdminHandler_DeleteQuestion_Success(t *testing.T) {
	h, categories := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	createBody := `{"id":"wedding","title":"Свадьба","base_prompt_template":"шаблон"}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(createBody)))

	addBody := `{"step_number":1,"question_text":"Как зовут жениха?","ui_type":"text","mapping_key":"GROOM","is_required":true}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/wedding/questions", strings.NewReader(addBody)))

	qid := categories.categories["wedding"].Questions[0].ID
	r := httptest.NewRequest(http.MethodDelete, "/admin/categories/wedding/questions/"+strconv.Itoa(qid), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d", w.Code)
	}
	if len(categories.categories["wedding"].Questions) != 0 {
		t.Errorf("вопрос должен быть удалён, осталось: %+v", categories.categories["wedding"].Questions)
	}
}

func TestAdminHandler_DeleteQuestion_NotFound(t *testing.T) {
	h, _ := newTestAdminHandlerWithCategories(t)
	router := adminTestRouter(h)

	createBody := `{"id":"wedding","title":"Свадьба","base_prompt_template":"шаблон"}`
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/admin/categories/", strings.NewReader(createBody)))

	r := httptest.NewRequest(http.MethodDelete, "/admin/categories/wedding/questions/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", w.Code)
	}
}
