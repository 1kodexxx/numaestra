// internal/delivery/http/order_handler.go
package apphttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
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
	// webhookAllowedNets — подсети, с которых принимается вебхук Robokassa.
	// Пустой список отключает IP-фильтрацию (остаётся только проверка подписи).
	webhookAllowedNets []*net.IPNet
}

func NewOrderHandler(uc *usecase.OrderUseCase, log *slog.Logger, rk *robokassa.Client, webhookAllowedNets []*net.IPNet) *OrderHandler {
	return &OrderHandler{
		uc:                 uc,
		log:                log,
		rk:                 rk,
		webhookAllowedNets: webhookAllowedNets,
	}
}

// ctxKey — приватный тип для ключей контекста, исключает коллизии с другими пакетами.
type ctxKey string

const ctxKeyOrder ctxKey = "order"

func (h *OrderHandler) Routes() chi.Router {
	r := chi.NewRouter()

	// Лимитеры создаются с независимыми бакетами, поэтому всплеск обычных
	// запросов не «съедает» лимит вебхука и наоборот.
	clientLimiter := RateLimiter(10, 20)
	// Вебхук Robokassa приходит с её IP и обслуживается отдельным, более щедрым
	// лимитером: иначе при исчерпании клиентского бакета вебхук получит 429,
	// и Robokassa начнёт его ретраить, задерживая зачисление оплаты.
	webhookLimiter := RateLimiter(20, 40)

	// Публичные маршруты — без токена, но под клиентским rate limiter.
	r.Group(func(r chi.Router) {
		r.Use(clientLimiter)
		r.Post("/", h.CreateOrder)
	})

	// Вебхук оплаты — отдельный лимитер, не зависящий от клиентского трафика,
	// плюс фильтрация по IP-подсетям Robokassa (если они сконфигурированы):
	// ResultURL должен приходить только с официальных адресов шлюза.
	r.Group(func(r chi.Router) {
		r.Use(IPAllowlist(h.webhookAllowedNets))
		r.Use(webhookLimiter)
		r.Post("/webhook/robokassa", h.HandleRobokassaWebhook)
	})

	// Защищённые маршруты — требуют X-Access-Token заголовок.
	r.Group(func(r chi.Router) {
		r.Use(clientLimiter)
		r.Use(h.requireOrderAccess)
		r.Get("/", h.ListOrders)
		r.Get("/{id}", h.GetOrder)
	})

	return r
}

// requireOrderAccess — middleware проверки токена доступа.
// Токен передаётся в заголовке X-Access-Token.
// При успехе кладёт найденный заказ в контекст для использования в хендлерах.
func (h *OrderHandler) requireOrderAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Access-Token")
		if token == "" {
			h.errorResponse(w, r, http.StatusUnauthorized, "требуется заголовок X-Access-Token")
			return
		}

		order, err := h.uc.GetOrderByToken(r.Context(), token)
		if err != nil {
			h.errorResponse(w, r, http.StatusUnauthorized, "неверный токен доступа")
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyOrder, order)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// --- DTO (Структуры для JSON) ---

type CreateOrderRequest struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
	Brief string `json:"brief"`
	// Plan — выбранный клиентом тариф. Цену по тарифу определяет сервер,
	// сумма НЕ принимается из запроса (защита от занижения цены).
	Plan string `json:"plan"`
}

type OrderResponse struct {
	ID               string `json:"id"`
	InvoiceID        int64  `json:"invoice_id"`
	PaymentStatus    string `json:"payment_status"`
	GenerationStatus string `json:"generation_status"`
	AmountKopecks    int64  `json:"amount_kopecks"` // итоговая цена, определённая сервером
	PaymentURL       string `json:"payment_url"`    // <-- Ссылка для редиректа клиента
	// AccessToken выдаётся клиенту один раз при создании заказа и предъявляется
	// в заголовке X-Access-Token для доступа к защищённым маршрутам заказа.
	AccessToken string `json:"access_token"`
}

// --- Обработчики ---

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	// Базовая валидация входных данных до обращения к use-case,
	// чтобы вернуть клиенту понятный 400 вместо общего 500.
	if req.Email == "" && req.Phone == "" {
		h.errorResponse(w, r, http.StatusBadRequest, "укажите email или телефон")
		return
	}
	if req.Brief == "" {
		h.errorResponse(w, r, http.StatusBadRequest, "поле brief обязательно")
		return
	}
	if utf8.RuneCountInString(req.Brief) > domain.MaxBriefLength {
		h.errorResponse(w, r, http.StatusBadRequest, "поле brief слишком длинное")
		return
	}

	order, err := h.uc.CreateOrder(r.Context(), req.Email, req.Phone, req.Brief, req.Plan)
	if err != nil {
		// Неизвестный тариф или слишком длинный бриф — ошибка клиента (400),
		// остальное — внутренняя (500).
		if errors.Is(err, usecase.ErrUnknownPlan) {
			h.errorResponse(w, r, http.StatusBadRequest, "неизвестный тариф")
			return
		}
		if errors.Is(err, domain.ErrBriefTooLong) {
			h.errorResponse(w, r, http.StatusBadRequest, "поле brief слишком длинное")
			return
		}
		// Внутреннюю причину не раскрываем клиенту — только логируем.
		h.log.Error("ошибка создания заказа", "err", err)
		h.errorResponse(w, r, http.StatusInternalServerError, "не удалось создать заказ")
		return
	}

	// Сумму берём из заказа (её определил сервер), а не из запроса клиента.
	outSum := robokassa.FormatAmount(order.AmountKopecks())
	description := "Генерация студийной песни Numaestra"
	paymentURL := h.rk.PaymentURL(outSum, order.InvoiceID(), description)

	res := OrderResponse{
		ID:               order.ID().String(),
		InvoiceID:        order.InvoiceID(),
		PaymentStatus:    string(order.PaymentStatus()),
		GenerationStatus: string(order.GenerationStatus()),
		AmountKopecks:    order.AmountKopecks(),
		PaymentURL:       paymentURL, // <-- Фронтенд получит эту ссылку и перенаправит юзера
		AccessToken:      order.AccessToken(),
	}

	h.successResponse(w, http.StatusCreated, res)
}

func (h *OrderHandler) HandleRobokassaWebhook(w http.ResponseWriter, r *http.Request) {
	// ParseForm обрабатывает и POST данные, и Query параметры
	if err := r.ParseForm(); err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "не удалось разобрать параметры")
		return
	}

	outSum := r.Form.Get("OutSum")
	invIdStr := r.Form.Get("InvId")
	signature := r.Form.Get("SignatureValue")

	if outSum == "" || invIdStr == "" || signature == "" {
		h.errorResponse(w, r, http.StatusBadRequest, "отсутствуют обязательные параметры")
		return
	}

	if !h.rk.VerifyWebhook(outSum, invIdStr, signature) {
		h.log.Warn("попытка подделки вебхука Робокассы!", "inv_id", invIdStr)
		h.errorResponse(w, r, http.StatusBadRequest, "неверная подпись")
		return
	}

	invoiceID, err := strconv.ParseInt(invIdStr, 10, 64)
	if err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "некорректный формат InvId")
		return
	}

	paidKopecks, err := robokassa.ParseAmountKopecks(outSum)
	if err != nil {
		h.log.Warn("некорректная сумма в вебхуке Робокассы", "inv_id", invIdStr, "out_sum", outSum, "err", err)
		h.errorResponse(w, r, http.StatusBadRequest, "некорректный формат OutSum")
		return
	}

	if err := h.uc.HandlePaymentSuccess(r.Context(), invoiceID, paidKopecks); err != nil {
		if errors.Is(err, usecase.ErrPaymentAmountMismatch) {
			h.log.Warn("отклонён вебхук: сумма оплаты не совпадает", "invoice_id", invoiceID, "out_sum", outSum)
			h.errorResponse(w, r, http.StatusBadRequest, "сумма оплаты не совпадает с суммой заказа")
			return
		}
		h.log.Error("ошибка обработки вебхука оплаты", "invoice_id", invoiceID, "err", err)
		h.errorResponse(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	// Робокасса требует жесткий формат ответа при успехе
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK" + invIdStr))
}

// errorResponse пишет JSON-ошибку и добавляет request_id из контекста запроса,
// чтобы клиент мог сослаться на конкретный запрос при обращении в поддержку.
func (h *OrderHandler) errorResponse(w http.ResponseWriter, r *http.Request, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	body := map[string]string{"error": msg}
	if reqID := chimiddleware.GetReqID(r.Context()); reqID != "" {
		body["request_id"] = reqID
	}
	json.NewEncoder(w).Encode(body)
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
	// Заказ уже загружен и проверен middleware requireOrderAccess.
	order := r.Context().Value(ctxKeyOrder).(*domain.Order)

	// Дополнительно проверяем что ID в URL совпадает с заказом из токена.
	idParam := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idParam)
	if err != nil {
		h.errorResponse(w, r, http.StatusBadRequest, "некорректный ID заказа")
		return
	}
	if order.ID() != orderID {
		h.errorResponse(w, r, http.StatusForbidden, "токен не соответствует этому заказу")
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

// ListOrders возвращает все заказы владельца, определённого по X-Access-Token.
// GET /api/v1/orders с заголовком X-Access-Token.
// Владелец берётся из заказа, найденного по токену: если у него указан email —
// возвращаются заказы по email, иначе по телефону.
func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	// Заказ из токена определяет владельца — возвращаем все его заказы по email или телефону.
	owner := r.Context().Value(ctxKeyOrder).(*domain.Order)

	var (
		orders []*domain.Order
		err    error
	)
	if owner.CustomerEmail() != "" {
		orders, err = h.uc.ListOrdersByEmail(r.Context(), owner.CustomerEmail())
	} else {
		orders, err = h.uc.ListOrdersByPhone(r.Context(), owner.CustomerPhone())
	}
	if err != nil {
		h.log.Error("ошибка получения списка заказов", "err", err)
		h.errorResponse(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
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
