package apphttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

// inMemReviewRepo — потокобезопасности не требует (тесты последовательны).
type inMemReviewRepo struct {
	items []*domain.Review
}

func newInMemReviewRepo() *inMemReviewRepo { return &inMemReviewRepo{} }

func (r *inMemReviewRepo) Create(_ context.Context, rev *domain.Review) error {
	r.items = append([]*domain.Review{rev}, r.items...) // новые первыми
	return nil
}

func (r *inMemReviewRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Review, error) {
	for _, it := range r.items {
		if it.ID() == id {
			return it, nil
		}
	}
	return nil, domain.ErrReviewNotFound
}

func (r *inMemReviewRepo) ListPublished(_ context.Context, _, _ int) ([]*domain.Review, error) {
	var out []*domain.Review
	for _, it := range r.items {
		if it.IsPublished() {
			out = append(out, it)
		}
	}
	return out, nil
}

func (r *inMemReviewRepo) CountPublished(_ context.Context) (int, error) {
	n := 0
	for _, it := range r.items {
		if it.IsPublished() {
			n++
		}
	}
	return n, nil
}

func (r *inMemReviewRepo) ListAll(_ context.Context, _, _ int) ([]*domain.Review, error) {
	return r.items, nil
}

func (r *inMemReviewRepo) Update(_ context.Context, rev *domain.Review) error {
	for i, it := range r.items {
		if it.ID() == rev.ID() {
			r.items[i] = rev
			return nil
		}
	}
	return domain.ErrReviewNotFound
}

func (r *inMemReviewRepo) Delete(_ context.Context, id uuid.UUID) error {
	for i, it := range r.items {
		if it.ID() == id {
			r.items = append(r.items[:i], r.items[i+1:]...)
			return nil
		}
	}
	return domain.ErrReviewNotFound
}

var _ domain.ReviewRepository = (*inMemReviewRepo)(nil)

func seedReview(t *testing.T, repo *inMemReviewRepo) *domain.Review {
	t.Helper()
	rev, err := domain.NewReview("Иван", 5, "Отличный сервис")
	if err != nil {
		t.Fatalf("NewReview: %v", err)
	}
	_ = repo.Create(context.Background(), rev)
	return rev
}

// --- Публичный хендлер ---

func TestReviewHandler_Create_Success(t *testing.T) {
	repo := newInMemReviewRepo()
	h := NewReviewHandler(usecase.NewReviewUseCase(repo, discardAdminLogger()), discardAdminLogger())
	router := chi.NewRouter()
	router.Mount("/reviews", h.Routes())

	body := `{"author_name":"Аня","rating":5,"body":"Супер, песня готова за 10 минут!"}`
	req := httptest.NewRequest(http.MethodPost, "/reviews/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ожидали 201, получили %d (%s)", rec.Code, rec.Body.String())
	}
	if len(repo.items) != 1 || repo.items[0].AuthorName() != "Аня" {
		t.Errorf("отзыв не сохранён: %+v", repo.items)
	}
}

func TestReviewHandler_Create_InvalidRating(t *testing.T) {
	repo := newInMemReviewRepo()
	h := NewReviewHandler(usecase.NewReviewUseCase(repo, discardAdminLogger()), discardAdminLogger())
	router := chi.NewRouter()
	router.Mount("/reviews", h.Routes())

	req := httptest.NewRequest(http.MethodPost, "/reviews/", strings.NewReader(`{"author_name":"Аня","rating":9,"body":"x"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("оценка 9 должна дать 400, получили %d", rec.Code)
	}
}

func TestReviewHandler_List_OnlyPublished(t *testing.T) {
	repo := newInMemReviewRepo()
	pub := seedReview(t, repo)
	hidden := seedReview(t, repo)
	hidden.SetPublished(false)

	h := NewReviewHandler(usecase.NewReviewUseCase(repo, discardAdminLogger()), discardAdminLogger())
	router := chi.NewRouter()
	router.Mount("/reviews", h.Routes())

	req := httptest.NewRequest(http.MethodGet, "/reviews/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", rec.Code)
	}
	var resp struct {
		Reviews []map[string]any `json:"reviews"`
		Total   int              `json:"total"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Total != 1 || len(resp.Reviews) != 1 || resp.Reviews[0]["id"] != pub.ID().String() {
		t.Errorf("публичный список должен содержать только опубликованный: %+v", resp)
	}
}

// --- Админский хендлер ---

func newAdminReviewRouter(repo domain.ReviewRepository) http.Handler {
	uc := usecase.NewAdminUseCase(newAdminOrderRepo(), newAdminAccRepo(), nil, &noopRefunder{}, nil, nil, discardAdminLogger())
	h := NewAdminHandler(uc, discardAdminLogger()).WithReviews(usecase.NewReviewUseCase(repo, discardAdminLogger()))
	return adminTestRouter(h)
}

func TestAdminHandler_ListReviews_IncludesHidden(t *testing.T) {
	repo := newInMemReviewRepo()
	seedReview(t, repo)
	hidden := seedReview(t, repo)
	hidden.SetPublished(false)

	router := newAdminReviewRouter(repo)
	req := httptest.NewRequest(http.MethodGet, "/admin/reviews/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", rec.Code)
	}
	var list []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Errorf("админ должен видеть все отзывы (вкл. скрытые), получили %d", len(list))
	}
}

func TestAdminHandler_ReplyReview(t *testing.T) {
	repo := newInMemReviewRepo()
	rev := seedReview(t, repo)
	router := newAdminReviewRouter(repo)

	req := httptest.NewRequest(http.MethodPost, "/admin/reviews/"+rev.ID().String()+"/reply", strings.NewReader(`{"message":"Спасибо за отзыв!"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d (%s)", rec.Code, rec.Body.String())
	}
	got, _ := repo.GetByID(context.Background(), rev.ID())
	if got.AdminReply() != "Спасибо за отзыв!" {
		t.Errorf("ответ не сохранён: %q", got.AdminReply())
	}
}

func TestAdminHandler_ReplyReview_NotFound(t *testing.T) {
	router := newAdminReviewRouter(newInMemReviewRepo())
	req := httptest.NewRequest(http.MethodPost, "/admin/reviews/"+uuid.New().String()+"/reply", strings.NewReader(`{"message":"hi"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидали 404, получили %d", rec.Code)
	}
}

func TestAdminHandler_SetReviewPublished(t *testing.T) {
	repo := newInMemReviewRepo()
	rev := seedReview(t, repo)
	router := newAdminReviewRouter(repo)

	req := httptest.NewRequest(http.MethodPatch, "/admin/reviews/"+rev.ID().String(), strings.NewReader(`{"is_published":false}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидали 200, получили %d", rec.Code)
	}
	got, _ := repo.GetByID(context.Background(), rev.ID())
	if got.IsPublished() {
		t.Error("отзыв должен стать скрытым")
	}
}

func TestAdminHandler_DeleteReview(t *testing.T) {
	repo := newInMemReviewRepo()
	rev := seedReview(t, repo)
	router := newAdminReviewRouter(repo)

	req := httptest.NewRequest(http.MethodDelete, "/admin/reviews/"+rev.ID().String(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("ожидали 204, получили %d", rec.Code)
	}
	if len(repo.items) != 0 {
		t.Error("отзыв должен быть удалён")
	}
}
