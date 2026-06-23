package apphttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

// --- in-memory ExampleRepository для HTTP-тестов ---

type adminExampleRepo struct {
	mu    sync.Mutex
	items map[string]domain.ExampleSnapshot
}

func newAdminExampleRepo() *adminExampleRepo {
	return &adminExampleRepo{items: make(map[string]domain.ExampleSnapshot)}
}

func (r *adminExampleRepo) GetAll(_ context.Context) ([]*domain.Example, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domain.Example, 0, len(r.items))
	for _, s := range r.items {
		out = append(out, domain.RestoreExample(s))
	}
	return out, nil
}

func (r *adminExampleRepo) GetActive(_ context.Context) ([]*domain.Example, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.Example
	for _, s := range r.items {
		if s.IsActive {
			out = append(out, domain.RestoreExample(s))
		}
	}
	return out, nil
}

func (r *adminExampleRepo) GetByID(_ context.Context, id string) (*domain.Example, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.items[id]
	if !ok {
		return nil, domain.ErrExampleNotFound
	}
	return domain.RestoreExample(s), nil
}

func (r *adminExampleRepo) Create(_ context.Context, e *domain.Example) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := e.Snapshot()
	if _, ok := r.items[s.ID]; ok {
		return domain.ErrExampleAlreadyExists
	}
	r.items[s.ID] = s
	return nil
}

func (r *adminExampleRepo) Update(_ context.Context, e *domain.Example) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := e.Snapshot()
	if _, ok := r.items[s.ID]; !ok {
		return domain.ErrExampleNotFound
	}
	r.items[s.ID] = s
	return nil
}

func (r *adminExampleRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return domain.ErrExampleNotFound
	}
	delete(r.items, id)
	return nil
}

var _ domain.ExampleRepository = (*adminExampleRepo)(nil)

func newTestAdminHandlerWithExamples(t *testing.T) (*AdminHandler, *adminExampleRepo) {
	t.Helper()
	repo := newAdminExampleRepo()
	uc := usecase.NewAdminUseCase(newAdminOrderRepo(), newAdminAccRepo(), nil, &noopRefunder{}, nil, nil, discardAdminLogger())
	exampleUC := usecase.NewExampleUseCase(repo, discardAdminLogger())
	h := NewAdminHandler(uc, discardAdminLogger()).WithExamples(exampleUC)
	return h, repo
}

func TestAdminHandler_Examples_CRUD(t *testing.T) {
	h, _ := newTestAdminHandlerWithExamples(t)
	router := adminTestRouter(h)

	// Create
	body := `{"id":"e1","title":"Пример","category":"Юбилей","mood":"Праздник","audio_url":"a.mp3","cover_url":"c.webp","sort_order":1,"is_active":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/examples/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: ожидали 201, получили %d (%s)", rec.Code, rec.Body.String())
	}

	// Duplicate → 409
	req = httptest.NewRequest(http.MethodPost, "/admin/examples/", strings.NewReader(body))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: ожидали 409, получили %d", rec.Code)
	}

	// List → 1
	req = httptest.NewRequest(http.MethodGet, "/admin/examples/", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: ожидали 200, получили %d", rec.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 || list[0]["id"] != "e1" {
		t.Fatalf("ожидали 1 пример e1, получили %+v", list)
	}

	// Update
	upd := `{"title":"Обновлён","category":"Свадьба","mood":"Тепло","audio_url":"a2.mp3","cover_url":"c2.webp","sort_order":2,"is_active":false}`
	req = httptest.NewRequest(http.MethodPut, "/admin/examples/e1", strings.NewReader(upd))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update: ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}

	// Update missing → 404
	req = httptest.NewRequest(http.MethodPut, "/admin/examples/missing", strings.NewReader(upd))
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update missing: ожидали 404, получили %d", rec.Code)
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/admin/examples/e1", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: ожидали 204, получили %d", rec.Code)
	}

	// Delete missing → 404
	req = httptest.NewRequest(http.MethodDelete, "/admin/examples/e1", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing: ожидали 404, получили %d", rec.Code)
	}
}

func TestAdminHandler_Examples_NotRegisteredWithoutBuilder(t *testing.T) {
	// Без WithExamples роуты примеров не регистрируются.
	uc := usecase.NewAdminUseCase(newAdminOrderRepo(), newAdminAccRepo(), nil, &noopRefunder{}, nil, nil, discardAdminLogger())
	router := adminTestRouter(NewAdminHandler(uc, discardAdminLogger()))

	req := httptest.NewRequest(http.MethodGet, "/admin/examples/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("без WithExamples ожидали 404, получили %d", rec.Code)
	}
}

func TestAdminHandler_UploadExampleCover_NoS3(t *testing.T) {
	// Без CoverUploader загрузка обложки примера отвечает 503.
	h, _ := newTestAdminHandlerWithExamples(t)
	router := adminTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/admin/examples/e1/cover", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ожидали 503 без S3, получили %d", rec.Code)
	}
}

// statsStub — заглушка orderStatsProvider для теста дашборда.
type statsStub struct{}

func (statsStub) Stats(_ context.Context) (domain.OrderStats, error) {
	return domain.OrderStats{TotalOrders: 3, PaidOrders: 2, RevenueKopecks: 400000, Completed: 1}, nil
}

func TestAdminHandler_GetStats(t *testing.T) {
	exRepo := newAdminExampleRepo()
	_ = exRepo.Create(context.Background(), mustExample(t, "e1", true))
	_ = exRepo.Create(context.Background(), mustExample(t, "e2", false))

	statsUC := usecase.NewStatsUseCase(statsStub{}, newAdminAccRepo(), newAdminCategoryRepo(), exRepo, discardAdminLogger())
	uc := usecase.NewAdminUseCase(newAdminOrderRepo(), newAdminAccRepo(), nil, &noopRefunder{}, nil, nil, discardAdminLogger())
	router := adminTestRouter(NewAdminHandler(uc, discardAdminLogger()).WithStats(statsUC))

	req := httptest.NewRequest(http.MethodGet, "/admin/stats", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	orders, _ := resp["orders"].(map[string]any)
	if orders["total"] != float64(3) || orders["revenue_kopecks"] != float64(400000) {
		t.Errorf("неверные order-агрегаты: %+v", orders)
	}
	if resp["examples_total"] != float64(2) || resp["examples_active"] != float64(1) {
		t.Errorf("неверные счётчики примеров: total=%v active=%v", resp["examples_total"], resp["examples_active"])
	}
}

func TestExampleHandler_GetActive(t *testing.T) {
	repo := newAdminExampleRepo()
	_ = repo.Create(context.Background(), mustExample(t, "vis", true))
	_ = repo.Create(context.Background(), mustExample(t, "hid", false))

	h := NewExampleHandler(usecase.NewExampleUseCase(repo, discardAdminLogger()), discardAdminLogger())
	router := chi.NewRouter()
	router.Mount("/examples", h.Routes())

	req := httptest.NewRequest(http.MethodGet, "/examples/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", rec.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 || list[0]["id"] != "vis" {
		t.Fatalf("публичный список должен содержать только активные: %+v", list)
	}
}

func mustExample(t *testing.T, id string, active bool) *domain.Example {
	t.Helper()
	e, err := domain.NewExample(id, "Пример "+id, "Кат", "опис", "Тепло", "a.mp3", "c.webp", 1, active)
	if err != nil {
		t.Fatalf("NewExample: %v", err)
	}
	return e
}
