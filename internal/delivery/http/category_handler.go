package apphttp

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/numaestra/numaestra/internal/usecase"
)

type CategoryHandler struct {
	promptUC usecase.PromptBuilder
	log      *slog.Logger
}

func NewCategoryHandler(promptUC usecase.PromptBuilder, log *slog.Logger) *CategoryHandler {
	return &CategoryHandler{
		promptUC: promptUC,
		log:      log,
	}
}

// Возвращает chi.Router для монтирования в main.go
func (h *CategoryHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Те же лимиты, что и для клиентских маршрутов заказов.
	r.Use(RateLimiter(10, 20))

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
		h.log.Error("ошибка при получении визарда категории", "category_id", id, "error", err)
		respondError(w, r, http.StatusNotFound, "категория не найдена")
		return
	}

	respondJSON(w, http.StatusOK, category)
}
