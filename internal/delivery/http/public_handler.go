package apphttp

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/numaestra/numaestra/internal/domain"
)

// PublicHandler отдаёт публичные настройки витрины (цена, версия согласия).
type PublicHandler struct {
	priceKopecks int64
}

func NewPublicHandler(priceKopecks int64) *PublicHandler {
	return &PublicHandler{priceKopecks: priceKopecks}
}

func (h *PublicHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/config", h.GetConfig)
	return r
}

type publicConfigResponse struct {
	PriceKopecks      int64  `json:"price_kopecks"`
	PriceLabel        string `json:"price_label"`
	ConsentDocVersion string `json:"consent_doc_version"`
}

func (h *PublicHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, publicConfigResponse{
		PriceKopecks:      h.priceKopecks,
		PriceLabel:        formatRubles(h.priceKopecks),
		ConsentDocVersion: domain.CurrentConsentDocVersion,
	})
}

func formatRubles(kopecks int64) string {
	rub := kopecks / 100
	if kopecks%100 == 0 {
		return fmt.Sprintf("%d ₽", rub)
	}
	return fmt.Sprintf("%d,%02d ₽", rub, kopecks%100)
}
