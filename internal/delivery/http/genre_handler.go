package apphttp

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/numaestra/numaestra/internal/usecase"
)

type GenreHandler struct {
	genreUC *usecase.GenreUseCase
	log     *slog.Logger
	rdb     *redis.Client
}

func NewGenreHandler(genreUC *usecase.GenreUseCase, log *slog.Logger) *GenreHandler {
	return &GenreHandler{genreUC: genreUC, log: log}
}

func (h *GenreHandler) WithRedis(rdb *redis.Client) *GenreHandler {
	h.rdb = rdb
	return h
}

func (h *GenreHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(APIRateLimiter(h.rdb, 120, time.Minute, 20, 40))
	r.Get("/", h.List)
	return r
}

// List возвращает справочник жанров. ?category_id= — только жанры, привязанные к категории.
// GET /api/v1/genres
func (h *GenreHandler) List(w http.ResponseWriter, r *http.Request) {
	categoryID := r.URL.Query().Get("category_id")
	genres, err := h.genreUC.List(r.Context(), categoryID, true)
	if err != nil {
		h.log.Error("ошибка получения жанров", "error", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	respondJSON(w, http.StatusOK, genres)
}
