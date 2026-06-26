package apphttp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

// PromoHandler обрабатывает публичные запросы к промокодам.
type PromoHandler struct {
	uc  *usecase.PromoUseCase
	log *slog.Logger
}

func NewPromoHandler(uc *usecase.PromoUseCase, log *slog.Logger) *PromoHandler {
	return &PromoHandler{uc: uc, log: log}
}

func (h *PromoHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(RateLimiter(30, 60))
	r.Get("/{code}", h.ValidatePromo)
	return r
}

type PromoValidateResponse struct {
	Code            string `json:"code"`
	DiscountType    string `json:"discount_type"`
	DiscountValue   int    `json:"discount_value"`
	DiscountKopecks int64  `json:"discount_kopecks"`
}

// GET /api/v1/promo/{code}?amount_kopecks=N
// Проверяет промокод и возвращает размер скидки. Публичный эндпоинт.
func (h *PromoHandler) ValidatePromo(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(chi.URLParam(r, "code"))
	if code == "" {
		respondError(w, r, http.StatusBadRequest, "укажите код промокода")
		return
	}

	promo, err := h.uc.ValidatePromoCode(r.Context(), code)
	if err != nil {
		if errors.Is(err, domain.ErrPromoCodeNotFound) || errors.Is(err, domain.ErrPromoCodeInvalid) {
			respondError(w, r, http.StatusNotFound, "промокод не найден или недействителен")
			return
		}
		h.log.Error("ошибка проверки промокода", "code", code, "err", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	var discountKopecks int64
	if amountStr := r.URL.Query().Get("amount_kopecks"); amountStr != "" {
		if amount, err := strconv.ParseInt(amountStr, 10, 64); err == nil && amount > 0 {
			discountKopecks = promo.Apply(amount)
		}
	}

	respondJSON(w, http.StatusOK, PromoValidateResponse{
		Code:            promo.Code(),
		DiscountType:    string(promo.GetDiscountType()),
		DiscountValue:   promo.DiscountValue(),
		DiscountKopecks: discountKopecks,
	})
}
