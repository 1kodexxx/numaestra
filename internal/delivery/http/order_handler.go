package apphttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
	"github.com/numaestra/numaestra/pkg/idempotency"
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
	idempotency        idempotency.Storer
	// rdb — опциональный Redis-клиент; при наличии включает распределённый rate
	// limiter и защиту вебхука от replay-атак.
	rdb *redis.Client
}

func NewOrderHandler(uc *usecase.OrderUseCase, log *slog.Logger, rk *robokassa.Client, webhookAllowedNets []*net.IPNet) *OrderHandler {
	return &OrderHandler{
		uc:                 uc,
		log:                log,
		rk:                 rk,
		webhookAllowedNets: webhookAllowedNets,
	}
}

// WithIdempotency подключает Redis-стор идемпотентности к POST /api/v1/orders.
// Если не вызван — эндпоинт работает без дедупликации ретраев.
func (h *OrderHandler) WithIdempotency(store idempotency.Storer) *OrderHandler {
	h.idempotency = store
	return h
}

// WithRedis подключает Redis-клиент, включая распределённый rate limiter и
// защиту вебхука Robokassa от replay-атак через Redis-нонс.
func (h *OrderHandler) WithRedis(rdb *redis.Client) *OrderHandler {
	h.rdb = rdb
	return h
}

// ctxKey — приватный тип для ключей контекста, исключает коллизии с другими пакетами.
type ctxKey string

const ctxKeyOrder ctxKey = "order"

func (h *OrderHandler) Routes() chi.Router {
	r := chi.NewRouter()

	var clientLimiter, webhookLimiter func(http.Handler) http.Handler
	if h.rdb != nil {
		clientLimiter = DistributedRateLimiter(h.rdb, 60, time.Minute)
		webhookLimiter = DistributedRateLimiter(h.rdb, 120, time.Minute)
	} else {
		clientLimiter = RateLimiter(10, 20)
		webhookLimiter = RateLimiter(20, 40)
	}

	r.Group(func(r chi.Router) {
		r.Use(clientLimiter)
		if h.idempotency != nil {
			r.Use(idempotencyMiddleware(h.idempotency))
		}
		r.Post("/", h.CreateOrder)
	})

	r.Group(func(r chi.Router) {
		r.Use(IPAllowlist(h.webhookAllowedNets))
		r.Use(webhookLimiter)
		r.Post("/webhook/robokassa", h.HandleRobokassaWebhook)
	})

	r.Group(func(r chi.Router) {
		r.Use(clientLimiter)
		r.Use(h.requireOrderAccess)
		r.Get("/", h.ListOrders)
		r.Get("/{id}", h.GetOrder)
	})

	return r
}

// requireOrderAccess — middleware проверки токена доступа.
func (h *OrderHandler) requireOrderAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Access-Token")
		if token == "" {
			respondError(w, r, http.StatusUnauthorized, "требуется заголовок X-Access-Token")
			return
		}

		order, err := h.uc.GetOrderByToken(r.Context(), token)
		if err != nil {
			respondError(w, r, http.StatusUnauthorized, "неверный токен доступа")
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyOrder, order)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// --- DTO ---

type CreateOrderRequest struct {
	Email      string            `json:"email"`
	Phone      string            `json:"phone"`
	Brief      string            `json:"brief"`
	CategoryID string            `json:"category_id"`
	Answers    map[string]string `json:"answers"`
}

type OrderResponse struct {
	ID               string `json:"id"`
	InvoiceID        int64  `json:"invoice_id"`
	PaymentStatus    string `json:"payment_status"`
	GenerationStatus string `json:"generation_status"`
	AmountKopecks    int64  `json:"amount_kopecks"`
	PaymentURL       string `json:"payment_url"`
	AccessToken      string `json:"access_token"`
}

// --- Обработчики ---

func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	if req.Email == "" && req.Phone == "" {
		respondError(w, r, http.StatusBadRequest, "укажите email или телефон")
		return
	}
	if req.Brief == "" {
		respondError(w, r, http.StatusBadRequest, "поле brief обязательно")
		return
	}
	if utf8.RuneCountInString(req.Brief) > domain.MaxBriefLength {
		respondError(w, r, http.StatusBadRequest, "поле brief слишком длинное")
		return
	}

	order, err := h.uc.CreateOrder(r.Context(), req.Email, req.Phone, req.Brief, req.CategoryID, req.Answers)
	if err != nil {
		if errors.Is(err, domain.ErrBriefTooLong) {
			respondError(w, r, http.StatusBadRequest, "поле brief слишком длинное")
			return
		}
		if errors.Is(err, usecase.ErrInvalidEmail) {
			respondError(w, r, http.StatusBadRequest, "некорректный формат email")
			return
		}
		if errors.Is(err, usecase.ErrInvalidPhone) {
			respondError(w, r, http.StatusBadRequest, "некорректный формат телефона")
			return
		}
		h.log.Error("ошибка создания заказа", "err", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось создать заказ")
		return
	}

	outSum := robokassa.FormatAmount(order.AmountKopecks())
	description := "Генерация студийной песни Numaestra"
	paymentURL := h.rk.PaymentURL(outSum, order.InvoiceID(), description)

	respondJSON(w, http.StatusCreated, OrderResponse{
		ID:               order.ID().String(),
		InvoiceID:        order.InvoiceID(),
		PaymentStatus:    string(order.PaymentStatus()),
		GenerationStatus: string(order.GenerationStatus()),
		AmountKopecks:    order.AmountKopecks(),
		PaymentURL:       paymentURL,
		AccessToken:      order.AccessToken(),
	})
}

func (h *OrderHandler) HandleRobokassaWebhook(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respondError(w, r, http.StatusBadRequest, "не удалось разобрать параметры")
		return
	}

	outSum := r.Form.Get("OutSum")
	invIdStr := r.Form.Get("InvId")
	signature := r.Form.Get("SignatureValue")

	if outSum == "" || invIdStr == "" || signature == "" {
		respondError(w, r, http.StatusBadRequest, "отсутствуют обязательные параметры")
		return
	}

	if !h.rk.VerifyWebhook(outSum, invIdStr, signature) {
		h.log.Warn("попытка подделки вебхука Робокассы!", "inv_id", invIdStr)
		respondError(w, r, http.StatusBadRequest, "неверная подпись")
		return
	}

	// Защита от replay-атак: подпись валидна, но запрос уже обработан.
	// Проверяем Redis-нонс ПОСЛЕ проверки подписи, чтобы не давать оракул для перебора.
	if h.rdb != nil {
		nonceKey := fmt.Sprintf("webhook:seen:%s", invIdStr)
		exists, _ := h.rdb.Exists(r.Context(), nonceKey).Result()
		if exists > 0 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK" + invIdStr)) //nolint:errcheck
			return
		}
	}

	invoiceID, err := strconv.ParseInt(invIdStr, 10, 64)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный формат InvId")
		return
	}

	paidKopecks, err := robokassa.ParseAmountKopecks(outSum)
	if err != nil {
		h.log.Warn("некорректная сумма в вебхуке Робокассы", "inv_id", invIdStr, "out_sum", outSum, "err", err)
		respondError(w, r, http.StatusBadRequest, "некорректный формат OutSum")
		return
	}

	if err := h.uc.HandlePaymentSuccess(r.Context(), invoiceID, paidKopecks); err != nil {
		if errors.Is(err, usecase.ErrPaymentAmountMismatch) {
			h.log.Warn("отклонён вебхук: сумма оплаты не совпадает", "invoice_id", invoiceID, "out_sum", outSum)
			respondError(w, r, http.StatusBadRequest, "сумма оплаты не совпадает с суммой заказа")
			return
		}
		if errors.Is(err, usecase.ErrPaymentWindowExpired) {
			h.log.Warn("отклонён вебхук: платёжное окно истекло", "invoice_id", invoiceID)
			respondError(w, r, http.StatusBadRequest, "платёжное окно заказа истекло")
			return
		}
		h.log.Error("ошибка обработки вебхука оплаты", "invoice_id", invoiceID, "err", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	// Фиксируем нонс: повторный вебхук с тем же InvId будет отклонён быстро.
	if h.rdb != nil {
		nonceKey := fmt.Sprintf("webhook:seen:%s", invIdStr)
		h.rdb.Set(r.Context(), nonceKey, 1, 73*time.Hour) //nolint:errcheck
	}

	// Robokassa требует строгий формат ответа при успехе.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK" + invIdStr)) //nolint:errcheck
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
	order := r.Context().Value(ctxKeyOrder).(*domain.Order)

	idParam := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idParam)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID заказа")
		return
	}
	if order.ID() != orderID {
		respondError(w, r, http.StatusForbidden, "токен не соответствует этому заказу")
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

	respondJSON(w, http.StatusOK, OrderDetailResponse{
		ID:               order.ID().String(),
		Brief:            order.Brief(),
		PaymentStatus:    string(order.PaymentStatus()),
		GenerationStatus: string(order.GenerationStatus()),
		Tracks:           tracks,
	})
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
// Поддерживает пагинацию через query-параметры limit (default 20, max 100) и offset.
func (h *OrderHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	owner := r.Context().Value(ctxKeyOrder).(*domain.Order)

	limit, offset := parsePagination(r)

	var (
		orders []*domain.Order
		err    error
	)
	if owner.CustomerEmail() != "" {
		orders, err = h.uc.ListOrdersByEmail(r.Context(), owner.CustomerEmail(), limit, offset)
	} else {
		orders, err = h.uc.ListOrdersByPhone(r.Context(), owner.CustomerPhone(), limit, offset)
	}
	if err != nil {
		h.log.Error("ошибка получения списка заказов", "err", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
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

	respondJSON(w, http.StatusOK, res)
}

// parsePagination читает ?limit=&offset= из запроса.
// Ограничения: limit от 1 до 100, по умолчанию 20; offset >= 0.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 20
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		if v > 100 {
			v = 100
		}
		limit = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}
	return
}
