package apphttp

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/numaestra/numaestra/internal/domain"
)

// PublicHandler отдаёт публичные настройки витрины (цены, версия согласия).
type PublicHandler struct {
	priceKopecks     int64
	oldPriceKopecks  int64
	demoEnabled      bool
	demoPriceKopecks int64
}

func NewPublicHandler(priceKopecks, oldPriceKopecks int64, demoEnabled bool) *PublicHandler {
	return &PublicHandler{priceKopecks: priceKopecks, oldPriceKopecks: oldPriceKopecks, demoEnabled: demoEnabled}
}

// WithDemoPrice задаёт цену платного демо для витрины. 0 → демо бесплатное.
func (h *PublicHandler) WithDemoPrice(kopecks int64) *PublicHandler {
	h.demoPriceKopecks = kopecks
	return h
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
	OldPriceLabel string `json:"old_price_label,omitempty"`
	// DemoEnabled — доступно ли демо; false → фронт показывает воронку «оплата
	// сразу» одним платежом, без упоминания демо.
	DemoEnabled bool `json:"demo_enabled"`
	// Цена демо. 0 / пустая метка — демо бесплатное.
	DemoPriceKopecks int64  `json:"demo_price_kopecks"`
	DemoPriceLabel   string `json:"demo_price_label,omitempty"`
	// RemainingPriceLabel — сколько остаётся доплатить после демо (цена минус
	// демо). Витрина показывает эту сумму во втором шаге воронки, чтобы клиент
	// видел: суммарно он платит ровно цену заказа, а не сверх неё.
	RemainingPriceLabel string `json:"remaining_price_label,omitempty"`
	ConsentDocVersion   string `json:"consent_doc_version"`
}

func (h *PublicHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	resp := publicConfigResponse{
		PriceKopecks:      h.priceKopecks,
		PriceLabel:        formatRubles(h.priceKopecks),
		DemoEnabled:       h.demoEnabled,
		ConsentDocVersion: domain.CurrentConsentDocVersion,
	}
	// Старая цена имеет смысл, только когда она выше текущей.
	if h.oldPriceKopecks > h.priceKopecks {
		resp.OldPriceLabel = formatRubles(h.oldPriceKopecks)
	}
	// Цену демо показываем только когда она строго дешевле заказа — ровно то
	// условие, при котором сервер выставляет второй счёт (см. CreateOrder).
	if h.demoEnabled && h.demoPriceKopecks > 0 && h.priceKopecks > h.demoPriceKopecks {
		resp.DemoPriceKopecks = h.demoPriceKopecks
		resp.DemoPriceLabel = formatRubles(h.demoPriceKopecks)
		resp.RemainingPriceLabel = formatRubles(h.priceKopecks - h.demoPriceKopecks)
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
