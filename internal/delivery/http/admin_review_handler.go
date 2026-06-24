package apphttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
)

type adminReviewResponse struct {
	ID           string `json:"id"`
	AuthorName   string `json:"author_name"`
	Rating       int    `json:"rating"`
	Body         string `json:"body"`
	AdminReply   string `json:"admin_reply"`
	AdminReplyAt string `json:"admin_reply_at,omitempty"`
	IsPublished  bool   `json:"is_published"`
	CreatedAt    string `json:"created_at"`
}

func reviewToAdminResponse(r *domain.Review) adminReviewResponse {
	resp := adminReviewResponse{
		ID:          r.ID().String(),
		AuthorName:  r.AuthorName(),
		Rating:      r.Rating(),
		Body:        r.Body(),
		AdminReply:  r.AdminReply(),
		IsPublished: r.IsPublished(),
		CreatedAt:   r.CreatedAt().Format("2006-01-02T15:04:05Z"),
	}
	if r.AdminReplyAt() != nil {
		resp.AdminReplyAt = r.AdminReplyAt().Format("2006-01-02T15:04:05Z")
	}
	return resp
}

// ListReviews — GET /api/v1/admin/reviews (все, включая скрытые).
func (h *AdminHandler) ListReviews(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	reviews, err := h.reviewUC.ListAll(r.Context(), page, perPage)
	if err != nil {
		h.log.Error("admin: ошибка получения отзывов", "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить отзывы")
		return
	}
	resp := make([]adminReviewResponse, 0, len(reviews))
	for _, rev := range reviews {
		resp = append(resp, reviewToAdminResponse(rev))
	}
	respondJSON(w, http.StatusOK, resp)
}

type reviewReplyRequest struct {
	Message string `json:"message"`
}

// ReplyReview — POST /api/v1/admin/reviews/{id}/reply
func (h *AdminHandler) ReplyReview(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID отзыва")
		return
	}
	var req reviewReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	rev, err := h.reviewUC.Reply(r.Context(), id, req.Message)
	if err != nil {
		if errors.Is(err, domain.ErrReviewNotFound) {
			respondError(w, r, http.StatusNotFound, "отзыв не найден")
			return
		}
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, reviewToAdminResponse(rev))
}

type reviewPublishRequest struct {
	IsPublished bool `json:"is_published"`
}

// SetReviewPublished — PATCH /api/v1/admin/reviews/{id} (скрыть/показать).
func (h *AdminHandler) SetReviewPublished(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID отзыва")
		return
	}
	var req reviewPublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	rev, err := h.reviewUC.SetPublished(r.Context(), id, req.IsPublished)
	if err != nil {
		if errors.Is(err, domain.ErrReviewNotFound) {
			respondError(w, r, http.StatusNotFound, "отзыв не найден")
			return
		}
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, reviewToAdminResponse(rev))
}

// DeleteReview — DELETE /api/v1/admin/reviews/{id}
func (h *AdminHandler) DeleteReview(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID отзыва")
		return
	}
	if err := h.reviewUC.Delete(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrReviewNotFound) {
			respondError(w, r, http.StatusNotFound, "отзыв не найден")
			return
		}
		h.log.Error("admin: ошибка удаления отзыва", "review_id", id, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось удалить отзыв")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
