package apphttp

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

// ReviewHandler — публичные отзывы о приложении: список и создание без регистрации.
type ReviewHandler struct {
	uc  *usecase.ReviewUseCase
	log *slog.Logger
	rdb *redis.Client
}

func NewReviewHandler(uc *usecase.ReviewUseCase, log *slog.Logger) *ReviewHandler {
	return &ReviewHandler{uc: uc, log: log}
}

func (h *ReviewHandler) WithRedis(rdb *redis.Client) *ReviewHandler {
	h.rdb = rdb
	return h
}

func (h *ReviewHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.With(APIRateLimiter(h.rdb, 120, time.Minute, 10, 20)).Get("/", h.List)
	// Создание отзыва жёстче ограничено по частоте — защита от спама без регистрации.
	r.With(APIRateLimiter(h.rdb, 10, time.Minute, 1, 3)).Post("/", h.Create)
	return r
}

type reviewResponse struct {
	ID           string `json:"id"`
	AuthorName   string `json:"author_name"`
	Rating       int    `json:"rating"`
	Body         string `json:"body"`
	AdminReply   string `json:"admin_reply,omitempty"`
	AdminReplyAt string `json:"admin_reply_at,omitempty"`
	CreatedAt    string `json:"created_at"`
}

func reviewToResponse(r *domain.Review) reviewResponse {
	resp := reviewResponse{
		ID:         r.ID().String(),
		AuthorName: r.AuthorName(),
		Rating:     r.Rating(),
		Body:       r.Body(),
		AdminReply: r.AdminReply(),
		CreatedAt:  r.CreatedAt().Format("2006-01-02T15:04:05Z"),
	}
	if r.AdminReplyAt() != nil {
		resp.AdminReplyAt = r.AdminReplyAt().Format("2006-01-02T15:04:05Z")
	}
	return resp
}

type createReviewRequest struct {
	AuthorName string `json:"author_name"`
	Rating     int    `json:"rating"`
	Body       string `json:"body"`
}

// List — GET /api/v1/reviews?page=&per_page=
func (h *ReviewHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	reviews, total, err := h.uc.ListPublished(r.Context(), page, perPage)
	if err != nil {
		h.log.Error("ошибка получения отзывов", "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить отзывы")
		return
	}
	resp := make([]reviewResponse, 0, len(reviews))
	for _, rev := range reviews {
		resp = append(resp, reviewToResponse(rev))
	}
	respondJSON(w, http.StatusOK, map[string]any{"reviews": resp, "total": total})
}

// Create — POST /api/v1/reviews
func (h *ReviewHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	rev, err := h.uc.Create(r.Context(), req.AuthorName, req.Rating, req.Body)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, reviewToResponse(rev))
}
