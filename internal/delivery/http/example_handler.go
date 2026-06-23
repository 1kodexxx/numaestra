package apphttp

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/numaestra/numaestra/internal/usecase"
)

// ExampleHandler отдаёт публичный список примеров готовых работ для главной.
type ExampleHandler struct {
	uc  *usecase.ExampleUseCase
	log *slog.Logger
}

func NewExampleHandler(uc *usecase.ExampleUseCase, log *slog.Logger) *ExampleHandler {
	return &ExampleHandler{uc: uc, log: log}
}

func (h *ExampleHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(RateLimiter(10, 20))
	r.Get("/", h.HandleGetActive)
	return r
}

// HandleGetActive возвращает активные примеры (по sort_order).
// GET /api/v1/examples
func (h *ExampleHandler) HandleGetActive(w http.ResponseWriter, r *http.Request) {
	examples, err := h.uc.ListActive(r.Context())
	if err != nil {
		h.log.Error("ошибка получения примеров", "error", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	// Примеры меняются редко — разрешаем кеш на 1 час, как и категориям.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	respondJSON(w, http.StatusOK, examples)
}
