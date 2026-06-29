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
	// storage — хранилище треков; нужно для presigned-ссылок в ответах с треками.
	// nil → ссылки отдаются как сохранены в БД (как раньше, без presign).
	storage domain.TrackStorage
	// presignTTL — срок действия presigned-ссылки на трек (если presign включён).
	presignTTL time.Duration
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

// WithTrackStorage подключает хранилище треков и TTL для presigned-ссылок. Если
// не вызван (или presign в хранилище выключен) — ссылки на треки отдаются как
// сохранены в БД (постоянные публичные URL, как раньше).
func (h *OrderHandler) WithTrackStorage(storage domain.TrackStorage, presignTTL time.Duration) *OrderHandler {
	h.storage = storage
	h.presignTTL = presignTTL
	return h
}

// resolvePlayURL подписывает одиночную ссылку для клиента (трек или демо). При
// выключенном presign / отсутствии хранилища возвращает сохранённый URL как есть;
// при ошибке подписи деградирует до сохранённого URL, чтобы не ронять плеер.
func (h *OrderHandler) resolvePlayURL(ctx context.Context, storedURL string) string {
	if storedURL == "" || h.storage == nil {
		return storedURL
	}
	resolved, err := h.storage.ResolvePlayURL(ctx, storedURL, h.presignTTL)
	if err != nil {
		h.log.Warn("presign: не удалось подписать ссылку", "err", err)
		return storedURL
	}
	return resolved
}

// trackResponses строит ответы по трекам, подставляя presigned-ссылку для каждого.
func (h *OrderHandler) trackResponses(ctx context.Context, tracks []domain.Track) []TrackResponse {
	out := make([]TrackResponse, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, TrackResponse{
			Index:       t.Index,
			AudioURL:    h.resolvePlayURL(ctx, t.AudioURL),
			DurationSec: t.DurationSec,
		})
	}
	return out
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
		// Резолв UUID заказа по номеру счёта — для PaymentReturnPage на новом устройстве.
		// Жёсткий лимит 10/час/IP: InvId последователен, и без этого лимита перебором
		// можно собирать UUID существующих заказов. Легитимный трафик сюда единичный
		// (вызывается только как fallback, когда нет invoice-map в localStorage).
		r.With(APIRateLimiter(h.rdb, 10, time.Hour, 0.1, 10)).
			Get("/by-invoice/{invoiceId}", h.GetOrderByInvoice)
	})

	r.Group(func(r chi.Router) {
		r.Use(IPAllowlist(h.webhookAllowedNets))
		r.Use(webhookLimiter)
		r.Post("/webhook/robokassa", h.HandleRobokassaWebhook)
	})

	// sync-payment публичный: Robokassa — источник истины, токен не нужен.
	r.Group(func(r chi.Router) {
		r.Use(clientLimiter)
		r.Post("/{id}/sync-payment", h.SyncPayment)
	})

	r.Group(func(r chi.Router) {
		r.Use(clientLimiter)
		r.Use(h.requireOrderAccess)
		r.Get("/", h.ListOrders)
		r.Get("/{id}", h.GetOrder)
		r.Get("/{id}/payment-url", h.GetPaymentURL)
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
	PromoCode         string            `json:"promo_code,omitempty"`
	ReferralCode      string            `json:"referral_code,omitempty"`
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

	if req.Email == "" {
		respondError(w, r, http.StatusBadRequest, "укажите email")
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

	order, err := h.uc.CreateOrder(r.Context(), req.Email, req.Phone, req.Brief, req.CategoryID, req.ConsentDocVersion, req.PromoCode, req.ReferralCode, req.Answers)
	if err != nil {
		if errors.Is(err, domain.ErrBriefTooLong) {
			respondError(w, r, http.StatusBadRequest, "поле brief слишком длинное")
			return
		}
		if errors.Is(err, domain.ErrPromptTooLong) {
			respondError(w, r, http.StatusBadRequest, "слишком длинные ответы квиза")
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
		if errors.Is(err, domain.ErrPromoCodeNotFound) {
			respondError(w, r, http.StatusBadRequest, "промокод не найден")
			return
		}
		if errors.Is(err, domain.ErrPromoCodeInvalid) {
			respondError(w, r, http.StatusBadRequest, "промокод недействителен или исчерпан")
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

	// Запускаем бесплатное демо в фоне (best-effort, отдельная полоса). Ошибка
	// постановки не влияет ни на заказ, ни на оплату — клиент в худшем случае
	// просто не увидит демо и оплатит как обычно. clientIP — для суточного
	// лимита демо на IP (защита от выжигания дневного бюджета одним источником).
	if err := h.uc.TriggerDemo(r.Context(), order.ID(), clientIP(r)); err != nil {
		h.log.Warn("не удалось поставить задачу демо", "order_id", order.ID(), "err", err)
	}

	// Бесплатный заказ (промокод 100% скидки): Robokassa отклоняет 0 ₽ — применяем оплату сразу.
	if order.AmountKopecks() == 0 {
		if err := h.uc.HandlePaymentSuccess(r.Context(), order.InvoiceID(), 0); err != nil {
			h.log.Error("ошибка активации бесплатного заказа", "order_id", order.ID(), "err", err)
			respondError(w, r, http.StatusInternalServerError, "не удалось активировать заказ")
			return
		}
		// HandlePaymentSuccess работает с собственной копией заказа, поэтому локальный
		// агрегат `order` хранит устаревший generation_status. Бесплатный заказ после
		// активации переходит в queued — возвращаем актуальное значение, а не stale.
		respondJSON(w, http.StatusCreated, OrderResponse{
			ID:               order.ID().String(),
			InvoiceID:        order.InvoiceID(),
			PaymentStatus:    string(domain.PaymentStatusPaid),
			GenerationStatus: string(domain.GenerationStatusQueued),
			AmountKopecks:    0,
			PaymentURL:       "",
			AccessToken:      order.AccessToken(),
		})
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

// markWebhookSeen помечает InvId как обработанный (быстрый replay-намёк). Источник
// истины всё равно payment_status в БД, поэтому потеря нонса при сбросе Redis
// безопасна. No-op без Redis.
func (h *OrderHandler) markWebhookSeen(ctx context.Context, invIdStr string) {
	if h.rdb == nil {
		return
	}
	h.rdb.Set(ctx, fmt.Sprintf("webhook:seen:%s", invIdStr), 1, 73*time.Hour) //nolint:errcheck
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

	invoiceID, err := strconv.ParseInt(invIdStr, 10, 64)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный формат InvId")
		return
	}

	// Защита от replay-атак: подпись валидна, но запрос мог быть уже обработан.
	// Источник истины — payment_status в БД, а не один лишь Redis-нонс: нонс мог
	// проставиться по сбою/коллизии, а заказ остаться pending — слепое «OK» по
	// нонсу тогда потеряло бы оплату. Поэтому грузим заказ и решаем по факту.
	order, dbErr := h.uc.GetOrderByInvoiceID(r.Context(), invoiceID)
	switch {
	case dbErr == nil && order.PaymentStatus() == domain.PaymentStatusPaid:
		// Уже оплачен — фиксируем нонс и отвечаем OK без повторной обработки.
		h.markWebhookSeen(r.Context(), invIdStr)
		h.log.Info("вебхук: заказ уже оплачен (replay)", "invoice_id", invoiceID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK" + invIdStr)) //nolint:errcheck
		return
	case errors.Is(dbErr, domain.ErrOrderNotFound):
		// Подпись валидна, но заказа нет — обрабатывать нечего. Отвечаем OK, чтобы
		// Robokassa не уходила в бесконечные ретраи на заведомо неактивируемый InvId.
		h.log.Warn("вебхук: заказ по InvId не найден", "invoice_id", invIdStr)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK" + invIdStr)) //nolint:errcheck
		return
	case dbErr != nil:
		// Транзиентная ошибка БД: НЕ отвечаем OK (иначе потеряем платёж — Robokassa
		// перестанет ретраить). Возвращаем 500, доставка повторится.
		h.log.Error("вебхук: ошибка загрузки заказа по InvId", "invoice_id", invIdStr, "err", dbErr)
		respondError(w, r, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	// Заказ найден и ещё pending — активируем (HandlePaymentSuccess идемпотентен).

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

	// Фиксируем нонс: повторный вебхук с тем же InvId обработается через быстрый
	// paid-барьер выше.
	h.markWebhookSeen(r.Context(), invIdStr)

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
	// Демо-фрагмент (до оплаты). DemoStatus: none|processing|ready|failed.
	// DemoURL заполнен только при ready; фронт отдаёт его как стрим без скачивания.
	DemoStatus string `json:"demo_status"`
	DemoURL    string `json:"demo_url,omitempty"`
	// UpdatedAt — RFC3339 момент последнего изменения заказа. Пока заказ pending и
	// демо в processing, это момент старта демо — серверный якорь прогресса демо на
	// фронте (переживает перезагрузку, в отличие от привязки к монтированию).
	UpdatedAt string `json:"updated_at,omitempty"`
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

	tracks := h.trackResponses(r.Context(), order.Tracks())

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
		DemoStatus:         string(order.DemoStatus()),
		// DemoURL НЕ подписываем: демо — короткий тизер с водяным знаком, показывается
		// на pending-заказе, который активно опрашивается каждые 10с. Подпись меняла бы
		// demo_url на каждом опросе и сбрасывала бы воспроизведение демо. demos/* при
		// presign-режиме оставляем публичными (см. deploy/S3-PRESIGN.md).
		DemoURL:   order.DemoURL(),
		UpdatedAt: order.UpdatedAt().Format(time.RFC3339),
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
// Публичный эндпоинт: X-Access-Token не требуется — Robokassa сама является источником
// истины, поэтому риска выдачи лишних привилегий нет.
// POST /api/v1/orders/{id}/sync-payment
func (h *OrderHandler) SyncPayment(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный ID заказа")
		return
	}

	order, err := h.uc.GetOrder(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
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

	var paidKopecks int64
	if h.rk.IsTestAutoPay() {
		// Тестовый режим: пропускаем реальный запрос к Robokassa, используем сумму из БД.
		paidKopecks = order.AmountKopecks()
	} else {
		var paid bool
		var err error
		paidKopecks, paid, err = h.rk.GetPaidAmountKopecks(r.Context(), order.InvoiceID())
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
	}

	if err := h.uc.HandlePaymentSuccess(r.Context(), order.InvoiceID(), paidKopecks); err != nil {
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
	DemoStatus         string          `json:"demo_status"`
	DemoURL            string          `json:"demo_url,omitempty"`
	// UpdatedAt — момент старта демо (пока pending+processing), якорь прогресса демо.
	UpdatedAt string `json:"updated_at,omitempty"`
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

	tracks := h.trackResponses(r.Context(), order.Tracks())

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
		tracks = h.trackResponses(r.Context(), order.Tracks())
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
		DemoStatus:         string(order.DemoStatus()),
		// DemoURL НЕ подписываем: демо — короткий тизер с водяным знаком, показывается
		// на pending-заказе, который активно опрашивается каждые 10с. Подпись меняла бы
		// demo_url на каждом опросе и сбрасывала бы воспроизведение демо. demos/* при
		// presign-режиме оставляем публичными (см. deploy/S3-PRESIGN.md).
		DemoURL:   order.DemoURL(),
		UpdatedAt: order.UpdatedAt().Format(time.RFC3339),
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

// GetOrderByInvoice возвращает ID заказа по номеру счёта Robokassa (InvId).
// Используется PaymentReturnPage когда localStorage не содержит invoice map
// (другой браузер, очищенное хранилище, мобильный Safari в private mode):
// после возврата с оплаты страница резолвит UUID заказа по InvId и открывает
// его статус. Отдаёт только order UUID — без email/phone/brief.
//
// Безопасность: InvId последователен, поэтому endpoint жёстко ограничен по частоте
// (10 запросов/час/IP, см. Routes) — иначе перебором можно собирать UUID
// существующих заказов. Сделать «есть» и «нет» полностью неотличимыми нельзя:
// смысл endpoint'а — вернуть UUID существующего заказа, так что 200 vs 404
// неизбежны; именно rate-limit здесь основная защита. Невалидный и
// несуществующий InvId отвечают ОДИНАКОВО (404 «заказ не найден»), чтобы не
// раскрывать деталей разбора, а 404-промахи логируются как сигнал перебора.
// GET /api/v1/orders/by-invoice/{invoiceId}
func (h *OrderHandler) GetOrderByInvoice(w http.ResponseWriter, r *http.Request) {
	invParam := chi.URLParam(r, "invoiceId")
	invoiceID, err := strconv.ParseInt(invParam, 10, 64)
	if err != nil || invoiceID <= 0 {
		respondError(w, r, http.StatusNotFound, "заказ не найден")
		return
	}

	order, err := h.uc.GetOrderByInvoiceID(r.Context(), invoiceID)
	if err != nil {
		// Для этого endpoint'а 404 — основной сигнал перебора: легитимный
		// пользователь приходит с реальным InvId. Объём логов ограничен сверху
		// rate-limit'ом (≤10/час/IP), поэтому шума не создаёт.
		h.log.Warn("by-invoice: заказ не найден (возможен перебор InvId)",
			"invoice_id", invoiceID, "ip", clientIP(r))
		respondError(w, r, http.StatusNotFound, "заказ не найден")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"id": order.ID().String()})
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
