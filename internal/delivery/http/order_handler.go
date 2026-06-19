// internal/delivery/http/order_handler.go
package apphttp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/pkg/robokassa"
)

// OrderHandler обрабатывает HTTP-запросы, связанные с заказами.
type OrderHandler struct {
	uc  *usecase.OrderUseCase
	log *slog.Logger
	rk  *robokassa.Client
}

func NewOrderHandler(uc *usecase.OrderUseCase, log *slog.Logger, rk *robokassa.Client) *OrderHandler {
	return &OrderHandler{
		uc:  uc,
		log: log,
		rk:  rk,
	}
}

func (h *OrderHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/", h.CreateOrder)
	r.Get("/", h.ListOrders)
	r.Get("/{id}", h.GetOrder)
	r.Post("/webhook/robokassa", h.HandleRobokassaWebhook)

	return r
}

// --- DTO (Структуры для JSON) ---

type CreateOrderRequest struct {
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Brief         string `json:"brief"`
	AmountKopecks int64  `json:"amount_kopecks"`
}

type OrderResponse struct {
	ID               string `json:"id"`
	InvoiceID        int64  `json:"invoice_id"`
	PaymentStatus    string `json:"payment_status"`
	GenerationStatus string `json:"generation_status"`
	PaymentURL       string `json:"payment_url"` // <-- Ссылка для редиректа клиента
}

// --- Обработчики ---

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	order, err := h.uc.CreateOrder(r.Context(), req.Email, req.Phone, req.Brief, req.AmountKopecks)
	if err != nil {
		h.log.Error("ошибка создания заказа", "err", err)
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	outSum := robokassa.FormatAmount(req.AmountKopecks)
	description := "Генерация 4-х версий студийной песни Numaestra"
	paymentURL := h.rk.PaymentURL(outSum, order.InvoiceID(), description)

	res := OrderResponse{
		ID:               order.ID().String(),
		InvoiceID:        order.InvoiceID(),
		PaymentStatus:    string(order.PaymentStatus()),
		GenerationStatus: string(order.GenerationStatus()),
		PaymentURL:       paymentURL, // <-- Фронтенд получит эту ссылку и перенаправит юзера
	}

	h.successResponse(w, http.StatusCreated, res)
}

func (h *OrderHandler) HandleRobokassaWebhook(w http.ResponseWriter, r *http.Request) {
	// ParseForm обрабатывает и POST данные, и Query параметры
	if err := r.ParseForm(); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "не удалось разобрать параметры")
		return
	}

	outSum := r.Form.Get("OutSum")
	invIdStr := r.Form.Get("InvId")
	signature := r.Form.Get("SignatureValue")

	if outSum == "" || invIdStr == "" || signature == "" {
		h.errorResponse(w, http.StatusBadRequest, "отсутствуют обязательные параметры")
		return
	}

	if !h.rk.VerifyWebhook(outSum, invIdStr, signature) {
		h.log.Warn("попытка подделки вебхука Робокассы!", "inv_id", invIdStr)
		h.errorResponse(w, http.StatusBadRequest, "неверная подпись")
		return
	}

	invoiceID, err := strconv.ParseInt(invIdStr, 10, 64)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "некорректный формат InvId")
		return
	}

	if err := h.uc.HandlePaymentSuccess(r.Context(), invoiceID); err != nil {
		h.log.Error("ошибка обработки вебхука оплаты", "invoice_id", invoiceID, "err", err)
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Робокасса требует жесткий формат ответа при успехе
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK" + invIdStr))
}

func (h *OrderHandler) errorResponse(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (h *OrderHandler) successResponse(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

type TrackResponse struct {
	Index       int    `json:"index"`
	AudioURL    string `json:"audio_url"`
	DurationSec int    `json:"duration_sec"`
}

type OrderDetailResponse struct {
	ID               string          `json:"id"`
	Brief            string          `json:"brief"`
	PaymentStatus    string          `json:"payment_status"`
	GenerationStatus string          `json:"generation_status"`
	Tracks           []TrackResponse `json:"tracks,omitempty"`
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idParam)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "некорректный ID заказа")
		return
	}

	order, err := h.uc.GetOrder(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			h.errorResponse(w, http.StatusNotFound, "заказ не найден")
		} else {
			h.log.Error("ошибка получения заказа", "order_id", orderID, "err", err)
			h.errorResponse(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		}
		return
	}

	var tracks []TrackResponse
	for _, t := range order.Tracks() {
		tracks = append(tracks, TrackResponse{
			Index:       t.Index,
			AudioURL:    t.AudioURL,
			DurationSec: t.DurationSec,
		})
	}

	res := OrderDetailResponse{
		ID:               order.ID().String(),
		Brief:            order.Brief(),
		PaymentStatus:    string(order.PaymentStatus()),
		GenerationStatus: string(order.GenerationStatus()),
		Tracks:           tracks,
	}

	h.successResponse(w, http.StatusOK, res)
}

type OrderSummaryResponse struct {
	ID               string `json:"id"`
	InvoiceID        int64  `json:"invoice_id"`
	Brief            string `json:"brief"`
	PaymentStatus    string `json:"payment_status"`
	GenerationStatus string `json:"generation_status"`
	TracksCount      int    `json:"tracks_count"`
}

// ListOrders возвращает список заказов клиента по email или телефону.
// GET /api/v1/orders?email=user@example.com
// GET /api/v1/orders?phone=+79991234567
// Параметры взаимоисключающие; email имеет приоритет если указаны оба.
func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	phone := r.URL.Query().Get("phone")

	if email == "" && phone == "" {
		h.errorResponse(w, http.StatusBadRequest, "необходим параметр email или phone")
		return
	}

	var (
		orders []*domain.Order // domain imported above
		err    error
	)
	if email != "" {
		orders, err = h.uc.ListOrdersByEmail(r.Context(), email)
	} else {
		orders, err = h.uc.ListOrdersByPhone(r.Context(), phone)
	}
	if err != nil {
		h.log.Error("ошибка получения списка заказов", "email", email, "phone", phone, "err", err)
		h.errorResponse(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	res := make([]OrderSummaryResponse, 0, len(orders))
	for _, o := range orders {
		res = append(res, OrderSummaryResponse{
			ID:               o.ID().String(),
			InvoiceID:        o.InvoiceID(),
			Brief:            o.Brief(),
			PaymentStatus:    string(o.PaymentStatus()),
			GenerationStatus: string(o.GenerationStatus()),
			TracksCount:      len(o.Tracks()),
		})
	}

	h.successResponse(w, http.StatusOK, res)
}
