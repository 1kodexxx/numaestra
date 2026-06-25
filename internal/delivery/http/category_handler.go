package apphttp

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

type CategoryHandler struct {
	promptUC usecase.PromptBuilder
	log      *slog.Logger
	rdb      *redis.Client
}

func NewCategoryHandler(promptUC usecase.PromptBuilder, log *slog.Logger) *CategoryHandler {
	return &CategoryHandler{
		promptUC: promptUC,
		log:      log,
	}
}

func (h *CategoryHandler) WithRedis(rdb *redis.Client) *CategoryHandler {
	h.rdb = rdb
	return h
}

// Возвращает chi.Router для монтирования в main.go
func (h *CategoryHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Use(APIRateLimiter(h.rdb, 120, time.Minute, 10, 20))

	r.Get("/", h.HandleGetAll)
	r.Get("/{id}/wizard", h.HandleGetWizard)

	return r
}

func (h *CategoryHandler) HandleGetAll(w http.ResponseWriter, r *http.Request) {
	categories, err := h.promptUC.GetAllCategories(r.Context())
	if err != nil {
		h.log.Error("ошибка при получении категорий", "error", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	// Категории меняются редко — разрешаем браузерам и CDN кешировать на 1 час.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	respondJSON(w, http.StatusOK, categories)
}

func (h *CategoryHandler) HandleGetWizard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	category, err := h.promptUC.GetCategoryWizard(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrCategoryNotFound) {
			respondError(w, r, http.StatusNotFound, "категория не найдена")
			return
		}
		h.log.Error("ошибка при получении визарда категории", "category_id", id, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось загрузить категорию")
		return
	}

	respondJSON(w, http.StatusOK, toCategoryWizardResponse(category))
}

// categoryWizardJSON — публичный визард: вопросы квиза + шаблон для live-preview на фронте.
// base_prompt_template не отдаётся в списке категорий (MarshalJSON категории).
type categoryWizardJSON struct {
	ID                 string            `json:"id"`
	Title              string            `json:"title"`
	Description        string            `json:"description"`
	CoverImageURL      string            `json:"cover_image_url"`
	SeoTags            []string          `json:"seo_tags"`
	Questions          []domain.Question `json:"questions"`
	BasePromptTemplate string            `json:"base_prompt_template"`
}

func toCategoryWizardResponse(c *domain.Category) categoryWizardJSON {
	return categoryWizardJSON{
		ID:                 c.ID(),
		Title:              c.Title(),
		Description:        c.Description(),
		CoverImageURL:      c.CoverImageURL(),
		SeoTags:            c.SeoTags(),
		Questions:          c.Questions(),
		BasePromptTemplate: c.BasePromptTemplate(),
	}
}
