package apphttp

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/numaestra/numaestra/internal/usecase"
)

type CategoryHandler struct {
	promptUC *usecase.PromptUseCase
	log      *slog.Logger
}

// Теперь принимает UseCase и Logger, как и ожидает main.go
func NewCategoryHandler(promptUC *usecase.PromptUseCase, log *slog.Logger) *CategoryHandler {
	return &CategoryHandler{
		promptUC: promptUC,
		log:      log,
	}
}

// Возвращает chi.Router для монтирования в main.go
func (h *CategoryHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.HandleGetAll)
	r.Get("/{id}/wizard", h.HandleGetWizard)

	return r
}

func (h *CategoryHandler) HandleGetAll(w http.ResponseWriter, r *http.Request) {
	categories, err := h.promptUC.GetAllCategories(r.Context())
	if err != nil {
		h.log.Error("ошибка при получении категорий", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

func (h *CategoryHandler) HandleGetWizard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	category, err := h.promptUC.GetCategoryWizard(r.Context(), id)
	if err != nil {
		h.log.Error("ошибка при получении визарда категории", "category_id", id, "error", err)
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(category)
}
