package apphttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
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

	// Публичная страница «поделиться песней» — без X-Access-Token. ID заказа
	// (UUID, непредсказуем) играет роль capability-ссылки, как принято для
	// share-ссылок (Google Docs, Spotify и т.п.). Отдаёт только треки —
	// никаких email/phone/brief, чтобы шеринг не раскрывал личные данные.
	r.Group(func(r chi.Router) {
		r.Use(clientLimiter)
		r.Get("/{id}/share", h.GetPublicShare)
		r.Get("/{id}/status", h.GetPublicStatus)
		r.With(APIRateLimiter(h.rdb, 5, time.Hour, 1, 3)).
			Post("/{id}/access-link", h.RequestAccessLink)
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
		r.Get("/{id}/payment-url", h.GetPaymentURL)
		r.Post("/{id}/sync-payment", h.SyncPayment)
		r.Post("/{id}/share/revoke", h.RevokeShare)
		r.Post("/{id}/share/restore", h.RestoreShare)
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
	Email             string            `json:"email"`
	Phone             string            `json:"phone"`
	Brief             string            `json:"brief"`
	CategoryID        string            `json:"category_id"`
	ConsentDocVersion string            `json:"consent_doc_version"`
	Answers           map[string]string `json:"answers"`
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
	if err := domain.ValidateConsentDocVersion(req.ConsentDocVersion); err != nil {
		if errors.Is(err, domain.ErrInvalidConsentVersion) {
			respondError(w, r, http.StatusBadRequest, "устаревшая версия согласия, обновите страницу")
			return
		}
		respondError(w, r, http.StatusBadRequest, "необходимо согласие с условиями и обработкой персональных данных")
		return
	}

	order, err := h.uc.CreateOrder(r.Context(), req.Email, req.Phone, req.Brief, req.CategoryID, req.ConsentDocVersion, req.Answers)
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
		if errors.Is(err, domain.ErrCategoryNotFound) {
			respondError(w, r, http.StatusBadRequest, "категория не найдена")
			return
		}
		if errors.Is(err, domain.ErrMissingQuizAnswers) {
			respondError(w, r, http.StatusBadRequest, "не заполнены обязательные поля квиза")
			return
		}
		h.log.Error("ошибка создания заказа", "err", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось создать заказ")
		return
	}

	paymentURL := h.buildPaymentURL(order)

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
	h.log.Info("вебхук robokassa обработан", "invoice_id", invoiceID, "out_sum", outSum)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK" + invIdStr)) //nolint:errcheck
}

type TrackResponse struct {
	Index       int    `json:"index"`
	AudioURL    string `json:"audio_url"`
	DurationSec int    `json:"duration_sec"`
}

type OrderDetailResponse struct {
	ID                 string          `json:"id"`
	Brief              string          `json:"brief"`
	PaymentStatus      string          `json:"payment_status"`
	GenerationStatus   string          `json:"generation_status"`
	GenerationPhase    string          `json:"generation_phase,omitempty"`
	GenerationProgress int             `json:"generation_progress"`
	TracksReady        int             `json:"tracks_ready"`
	Tracks             []TrackResponse `json:"tracks,omitempty"`
	// PaidAt — момент оплаты (RFC3339), якорь для прогресс-бара генерации на
	// фронте. Пусто, пока заказ не оплачен.
	PaidAt       string `json:"paid_at,omitempty"`
	ShareRevoked bool   `json:"share_revoked"`
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	owner := r.Context().Value(ctxKeyOrder).(*domain.Order)

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID заказа")
		return
	}

	order, err := h.uc.GetOrderForCustomer(r.Context(), owner, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
			return
		}
		if errors.Is(err, domain.ErrOrderAccessDenied) {
			respondError(w, r, http.StatusForbidden, "нет доступа к этому заказу")
			return
		}
		h.log.Error("ошибка получения заказа", "order_id", orderID, "err", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
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

	paidAt := ""
	if t := order.PaidAt(); t != nil {
		paidAt = t.Format(time.RFC3339)
	}

	respondJSON(w, http.StatusOK, OrderDetailResponse{
		ID:                 order.ID().String(),
		Brief:              order.Brief(),
		PaymentStatus:      string(order.PaymentStatus()),
		GenerationStatus:   string(order.GenerationStatus()),
		GenerationPhase:    string(order.GenerationPhase()),
		GenerationProgress: order.GenerationProgress(),
		TracksReady:        order.TracksReady(),
		Tracks:             tracks,
		PaidAt:             paidAt,
		ShareRevoked:       order.ShareRevoked(),
	})
}

// buildPaymentURL формирует ссылку на оплату Robokassa для заказа.
func (h *OrderHandler) buildPaymentURL(order *domain.Order) string {
	outSum := robokassa.FormatAmount(order.AmountKopecks())
	return h.rk.PaymentURL(outSum, order.InvoiceID(), "Генерация студийной песни Numaestra")
}

// GetPaymentURL заново формирует ссылку на оплату для неоплаченного заказа —
// чтобы клиент мог повторить оплату со страницы статуса, если первая попытка
// сорвалась. Доступ по X-Access-Token (через requireOrderAccess).
// GET /api/v1/orders/{id}/payment-url
func (h *OrderHandler) GetPaymentURL(w http.ResponseWriter, r *http.Request) {
	owner := r.Context().Value(ctxKeyOrder).(*domain.Order)

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID заказа")
		return
	}

	order, err := h.uc.GetOrderForCustomer(r.Context(), owner, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
			return
		}
		if errors.Is(err, domain.ErrOrderAccessDenied) {
			respondError(w, r, http.StatusForbidden, "нет доступа к этому заказу")
			return
		}
		h.log.Error("ошибка получения заказа для оплаты", "order_id", orderID, "err", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	if order.PaymentStatus() != domain.PaymentStatusPending {
		respondError(w, r, http.StatusConflict, "заказ уже оплачен или недоступен для оплаты")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"payment_url": h.buildPaymentURL(order)})
}

// SyncPayment подтягивает статус оплаты из Robokassa, если ResultURL не дошёл.
// POST /api/v1/orders/{id}/sync-payment
func (h *OrderHandler) SyncPayment(w http.ResponseWriter, r *http.Request) {
	owner := r.Context().Value(ctxKeyOrder).(*domain.Order)

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID заказа")
		return
	}

	order, err := h.uc.GetOrderForCustomer(r.Context(), owner, orderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
			return
		}
		if errors.Is(err, domain.ErrOrderAccessDenied) {
			respondError(w, r, http.StatusForbidden, "нет доступа к этому заказу")
			return
		}
		h.log.Error("ошибка получения заказа для синхронизации оплаты", "order_id", orderID, "err", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	if order.PaymentStatus() == domain.PaymentStatusPaid {
		respondJSON(w, http.StatusOK, map[string]bool{"synced": true})
		return
	}
	if order.PaymentStatus() != domain.PaymentStatusPending {
		respondJSON(w, http.StatusOK, map[string]bool{"synced": false})
		return
	}

	paid, err := h.rk.IsInvoicePaid(r.Context(), order.InvoiceID())
	if err != nil {
		h.log.Warn("sync-payment: не удалось проверить оплату в Robokassa",
			"order_id", orderID, "invoice_id", order.InvoiceID(), "err", err)
		respondJSON(w, http.StatusOK, map[string]bool{"synced": false})
		return
	}
	if !paid {
		respondJSON(w, http.StatusOK, map[string]bool{"synced": false})
		return
	}

	if err := h.uc.HandlePaymentSuccess(r.Context(), order.InvoiceID(), order.AmountKopecks()); err != nil {
		if errors.Is(err, usecase.ErrPaymentAmountMismatch) || errors.Is(err, usecase.ErrPaymentWindowExpired) {
			h.log.Warn("sync-payment: оплата в Robokassa не применена", "order_id", orderID, "err", err)
			respondJSON(w, http.StatusOK, map[string]bool{"synced": false})
			return
		}
		h.log.Error("sync-payment: ошибка применения оплаты", "order_id", orderID, "err", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	h.log.Info("sync-payment: оплата подтверждена через OpStateExt", "order_id", orderID, "invoice_id", order.InvoiceID())
	respondJSON(w, http.StatusOK, map[string]bool{"synced": true})
}

type PublicShareResponse struct {
	ID     string          `json:"id"`
	Tracks []TrackResponse `json:"tracks"`
}

// PublicStatusResponse — публичный статус заказа по UUID (без email/brief/токена).
// Позволяет отслеживать заказ по ID из письма, даже если access_token утерян.
type PublicStatusResponse struct {
	ID                 string          `json:"id"`
	InvoiceID          int64           `json:"invoice_id"`
	PaymentStatus      string          `json:"payment_status"`
	GenerationStatus   string          `json:"generation_status"`
	GenerationPhase    string          `json:"generation_phase,omitempty"`
	GenerationProgress int             `json:"generation_progress"`
	TracksReady        int             `json:"tracks_ready"`
	PaidAt             string          `json:"paid_at,omitempty"`
	ShareRevoked       bool            `json:"share_revoked"`
	Tracks             []TrackResponse `json:"tracks,omitempty"`
}

// GetPublicShare отдаёт минимальный публичный вид завершённого заказа — только
// треки, без email/phone/brief/токена. Доступ без X-Access-Token: ID заказа
// (непредсказуемый UUID) сам по себе выступает capability-ссылкой для шеринга.
// GET /api/v1/orders/{id}/share
func (h *OrderHandler) GetPublicShare(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idParam)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID заказа")
		return
	}

	order, err := h.uc.GetOrder(r.Context(), orderID)
	if err != nil {
		respondError(w, r, http.StatusNotFound, "песня не найдена")
		return
	}
	// До завершения генерации публичной страницы не существует — не раскрываем
	// сам факт существования заказа в промежуточных статусах.
	if order.GenerationStatus() != domain.GenerationStatusCompleted {
		respondError(w, r, http.StatusNotFound, "песня не найдена")
		return
	}
	if order.ShareRevoked() {
		respondError(w, r, http.StatusNotFound, "песня не найдена")
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

	respondJSON(w, http.StatusOK, PublicShareResponse{
		ID:     order.ID().String(),
		Tracks: tracks,
	})
}

// GetPublicStatus отдаёт статус заказа без X-Access-Token. UUID заказа — capability-
// ссылка для отслеживания (как на странице «Статус заказа»). Без email/phone/brief.
// GET /api/v1/orders/{id}/status
func (h *OrderHandler) GetPublicStatus(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	orderID, err := uuid.Parse(idParam)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID заказа")
		return
	}

	order, err := h.uc.GetOrder(r.Context(), orderID)
	if err != nil {
		respondError(w, r, http.StatusNotFound, "заказ не найден")
		return
	}

	var tracks []TrackResponse
	if order.GenerationStatus() == domain.GenerationStatusCompleted {
		for _, t := range order.Tracks() {
			tracks = append(tracks, TrackResponse{
				Index:       t.Index,
				AudioURL:    t.AudioURL,
				DurationSec: t.DurationSec,
			})
		}
	}

	paidAt := ""
	if t := order.PaidAt(); t != nil {
		paidAt = t.Format(time.RFC3339)
	}

	respondJSON(w, http.StatusOK, PublicStatusResponse{
		ID:                 order.ID().String(),
		InvoiceID:          order.InvoiceID(),
		PaymentStatus:      string(order.PaymentStatus()),
		GenerationStatus:   string(order.GenerationStatus()),
		GenerationPhase:    string(order.GenerationPhase()),
		GenerationProgress: order.GenerationProgress(),
		TracksReady:        order.TracksReady(),
		PaidAt:             paidAt,
		ShareRevoked:       order.ShareRevoked(),
		Tracks:             tracks,
	})
}

type requestAccessLinkBody struct {
	Email string `json:"email"`
}

// RequestAccessLink отправляет на email ссылку с access_token, если email совпадает
// с заказом. Всегда отвечает одинаково — без раскрытия факта существования заказа.
// POST /api/v1/orders/{id}/access-link
func (h *OrderHandler) RequestAccessLink(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID заказа")
		return
	}

	var req requestAccessLinkBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	email := domain.NormalizeCustomerEmail(req.Email)
	if email == "" {
		respondError(w, r, http.StatusBadRequest, "укажите email")
		return
	}
	if _, err := mail.ParseAddress(email); err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный email")
		return
	}

	if err := h.uc.SendAccessLinkEmail(r.Context(), orderID, email); err != nil {
		h.log.Error("ошибка отправки ссылки доступа", "order_id", orderID, "err", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось отправить письмо")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Если заказ с указанным email существует, мы отправили ссылку для управления заказом. Проверьте почту и папку «Спам».",
	})
}

// RevokeShare отзывает публичную ссылку /s/{id} без смены UUID заказа.
// POST /api/v1/orders/{id}/share/revoke
func (h *OrderHandler) RevokeShare(w http.ResponseWriter, r *http.Request) {
	h.setShareRevoked(w, r, true)
}

// RestoreShare снова открывает публичную share-ссылку.
// POST /api/v1/orders/{id}/share/restore
func (h *OrderHandler) RestoreShare(w http.ResponseWriter, r *http.Request) {
	h.setShareRevoked(w, r, false)
}

func (h *OrderHandler) setShareRevoked(w http.ResponseWriter, r *http.Request, revoked bool) {
	owner := r.Context().Value(ctxKeyOrder).(*domain.Order)

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID заказа")
		return
	}

	if err := h.uc.SetShareRevoked(r.Context(), owner, orderID, revoked); err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
			return
		}
		if errors.Is(err, domain.ErrOrderAccessDenied) {
			respondError(w, r, http.StatusForbidden, "нет доступа к этому заказу")
			return
		}
		if errors.Is(err, domain.ErrShareNotAvailable) {
			respondError(w, r, http.StatusConflict, "публичная ссылка доступна только для готовых песен")
			return
		}
		h.log.Error("ошибка изменения share-ссылки", "order_id", orderID, "revoked", revoked, "err", err)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"share_revoked": revoked})
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
