// internal/delivery/http/order_handler.go
package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/numaestra/numaestra/internal/usecase"
)

// OrderHandler обрабатывает HTTP-запросы, связанные с заказами.
type OrderHandler struct {
	uc  *usecase.OrderUseCase
	log *slog.Logger
}

func NewOrderHandler(uc *usecase.OrderUseCase, log *slog.Logger) *OrderHandler {
	return &OrderHandler{
		uc:  uc,
		log: log,
	}
}

// Routes возвращает настроенный роутер для подмонтирования в main.go
func (h *OrderHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// API для клиентов (фронтенд)
	r.Post("/", h.CreateOrder)

	// API для сервисов (вебхуки кассы)
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
	// Здесь в будущем можно добавить ссылку на оплату Robokassa
}

// --- Обработчики ---

// CreateOrder принимает запрос на создание нового заказа.
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

	res := OrderResponse{
		ID:               order.ID().String(),
		InvoiceID:        order.InvoiceID(),
		PaymentStatus:    string(order.PaymentStatus()),
		GenerationStatus: string(order.GenerationStatus()),
	}

	h.successResponse(w, http.StatusCreated, res)
}

// HandleRobokassaWebhook симулирует прием успешной оплаты (ResultURL от Робокассы).
func (h *OrderHandler) HandleRobokassaWebhook(w http.ResponseWriter, r *http.Request) {
	// В реальной интеграции Робокасса шлет данные через POST form-urlencoded (x-www-form-urlencoded)
	// В MVP для простоты теста парсим из URL query parameters: ?InvId=12345
	invIdStr := r.URL.Query().Get("InvId")
	if invIdStr == "" {
		h.errorResponse(w, http.StatusBadRequest, "отсутствует параметр InvId")
		return
	}

	invoiceID, err := strconv.ParseInt(invIdStr, 10, 64)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, "некорректный формат InvId")
		return
	}

	// Вызываем бизнес-логику! UseCase сам поменяет статус, сохранит в БД и отправит в Asynq.
	if err := h.uc.HandlePaymentSuccess(r.Context(), invoiceID); err != nil {
		h.log.Error("ошибка обработки вебхука оплаты", "invoice_id", invoiceID, "err", err)
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Робокасса требует ответить просто "OK[InvId]" при успехе
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK" + invIdStr))
}

// --- Утилиты для ответов ---

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
