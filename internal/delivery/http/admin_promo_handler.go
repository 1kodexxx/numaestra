package apphttp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

// AdminPromoHandler — CRUD промокодов в административном API.
type AdminPromoHandler struct {
	uc  *usecase.PromoUseCase
	log *slog.Logger
}

func NewAdminPromoHandler(uc *usecase.PromoUseCase, log *slog.Logger) *AdminPromoHandler {
	return &AdminPromoHandler{uc: uc, log: log}
}

func (h *AdminPromoHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	return r
}

// --- DTOs ---

type promoResponse struct {
	ID            string  `json:"id"`
	Code          string  `json:"code"`
	DiscountType  string  `json:"discount_type"`
	DiscountValue int     `json:"discount_value"`
	MaxUses       *int    `json:"max_uses"`
	CurrentUses   int     `json:"current_uses"`
	ValidFrom     *string `json:"valid_from,omitempty"`
	ValidUntil    *string `json:"valid_until,omitempty"`
	IsActive      bool    `json:"is_active"`
	Description   string  `json:"description"`
	CreatedAt     string  `json:"created_at"`
}

func promoToResponse(p *domain.PromoCode) promoResponse {
	r := promoResponse{
		ID:            p.ID().String(),
		Code:          p.Code(),
		DiscountType:  string(p.GetDiscountType()),
		DiscountValue: p.DiscountValue(),
		MaxUses:       p.MaxUses(),
		CurrentUses:   p.CurrentUses(),
		IsActive:      p.Active(),
		Description:   p.Description(),
		CreatedAt:     p.CreatedAt().Format(time.RFC3339),
	}
	if t := p.ValidFrom(); t != nil {
		s := t.Format(time.RFC3339)
		r.ValidFrom = &s
	}
	if t := p.ValidUntil(); t != nil {
		s := t.Format(time.RFC3339)
		r.ValidUntil = &s
	}
	return r
}

type createPromoRequest struct {
	Code          string `json:"code"`
	DiscountType  string `json:"discount_type"`
	DiscountValue int    `json:"discount_value"`
	MaxUses       int    `json:"max_uses"`
	ValidFrom     string `json:"valid_from"`
	ValidUntil    string `json:"valid_until"`
	Description   string `json:"description"`
}

type updatePromoRequest struct {
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
	// MaxUses: опускается из JSON → nil (лимит не меняется); 0 → снять лимит; n>0 → установить n.
	MaxUses    *int   `json:"max_uses,omitempty"`
	ValidFrom  string `json:"valid_from"`
	ValidUntil string `json:"valid_until"`
}

// --- Handlers ---

func (h *AdminPromoHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	promos, total, err := h.uc.ListPromoCodes(r.Context(), limit, offset)
	if err != nil {
		h.log.Error("ошибка получения промокодов", "err", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	items := make([]promoResponse, 0, len(promos))
	for _, p := range promos {
		items = append(items, promoToResponse(p))
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *AdminPromoHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createPromoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	if req.Code == "" || req.DiscountType == "" || req.DiscountValue <= 0 {
		respondError(w, r, http.StatusBadRequest, "code, discount_type и discount_value обязательны")
		return
	}

	ucReq := usecase.CreatePromoRequest{
		Code: req.Code, DiscountType: req.DiscountType,
		DiscountValue: req.DiscountValue, MaxUses: req.MaxUses,
		Description: req.Description,
	}
	if req.ValidFrom != "" {
		if t, err := time.Parse(time.RFC3339, req.ValidFrom); err == nil {
			ucReq.ValidFrom = t
		}
	}
	if req.ValidUntil != "" {
		if t, err := time.Parse(time.RFC3339, req.ValidUntil); err == nil {
			ucReq.ValidUntil = t
		}
	}

	promo, err := h.uc.CreatePromoCode(r.Context(), ucReq)
	if err != nil {
		h.log.Error("ошибка создания промокода", "err", err)
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, promoToResponse(promo))
}

func (h *AdminPromoHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID")
		return
	}
	promo, err := h.uc.GetPromoCode(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrPromoCodeNotFound) {
			respondError(w, r, http.StatusNotFound, "промокод не найден")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	respondJSON(w, http.StatusOK, promoToResponse(promo))
}

func (h *AdminPromoHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID")
		return
	}
	var req updatePromoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	ucReq := usecase.UpdatePromoRequest{
		Description: req.Description,
		IsActive:    req.IsActive,
		MaxUses:     req.MaxUses,
	}
	if req.ValidFrom != "" {
		if t, err := time.Parse(time.RFC3339, req.ValidFrom); err == nil {
			ucReq.ValidFrom = t
		}
	}
	if req.ValidUntil != "" {
		if t, err := time.Parse(time.RFC3339, req.ValidUntil); err == nil {
			ucReq.ValidUntil = t
		}
	}

	promo, err := h.uc.UpdatePromoCode(r.Context(), id, ucReq)
	if err != nil {
		if errors.Is(err, domain.ErrPromoCodeNotFound) {
			respondError(w, r, http.StatusNotFound, "промокод не найден")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	respondJSON(w, http.StatusOK, promoToResponse(promo))
}

func (h *AdminPromoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID")
		return
	}
	if err := h.uc.DeletePromoCode(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrPromoCodeNotFound) {
			respondError(w, r, http.StatusNotFound, "промокод не найден")
			return
		}
		if errors.Is(err, domain.ErrPromoCodeInUse) {
			respondError(w, r, http.StatusConflict, "промокод используется в заказах и не может быть удалён")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}
