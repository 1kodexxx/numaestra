package apphttp

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/numaestra/numaestra/internal/domain"
)

// PublicHandler отдаёт публичные настройки витрины (цена, версия согласия).
type PublicHandler struct {
	priceKopecks    int64
	oldPriceKopecks int64
}

func NewPublicHandler(priceKopecks, oldPriceKopecks int64) *PublicHandler {
	return &PublicHandler{priceKopecks: priceKopecks, oldPriceKopecks: oldPriceKopecks}
}

func (h *PublicHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/config", h.GetConfig)
	return r
}

type publicConfigResponse struct {
	PriceKopecks int64  `json:"price_kopecks"`
	PriceLabel   string `json:"price_label"`
	// OldPriceLabel — зачёркнутая «старая» цена (маркетинг). Пусто = не показывать.
	OldPriceLabel     string `json:"old_price_label,omitempty"`
	ConsentDocVersion string `json:"consent_doc_version"`
}

func (h *PublicHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	resp := publicConfigResponse{
		PriceKopecks:      h.priceKopecks,
		PriceLabel:        formatRubles(h.priceKopecks),
		ConsentDocVersion: domain.CurrentConsentDocVersion,
	}
	// Старая цена имеет смысл, только когда она выше текущей.
	if h.oldPriceKopecks > h.priceKopecks {
		resp.OldPriceLabel = formatRubles(h.oldPriceKopecks)
	}
	respondJSON(w, http.StatusOK, resp)
}

func formatRubles(kopecks int64) string {
	rub := kopecks / 100
	if kopecks%100 == 0 {
		return fmt.Sprintf("%d ₽", rub)
	}
	return fmt.Sprintf("%d,%02d ₽", rub, kopecks%100)
}
