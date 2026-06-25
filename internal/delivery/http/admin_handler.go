package apphttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

// CoverUploader загружает обложку категории в объектное хранилище и возвращает
// постоянную публичную ссылку. Реализуется *s3.ResilientClient.
type CoverUploader interface {
	Upload(ctx context.Context, key, contentType string, data []byte) (string, error)
}

// AdminHandler обрабатывает запросы административного API.
// Все маршруты защищены Bearer-токеном через AdminAuth middleware.
type AdminHandler struct {
	uc        *usecase.AdminUseCase
	genreUC   *usecase.GenreUseCase   // nil → роуты жанров не регистрируются
	exampleUC *usecase.ExampleUseCase // nil → роуты примеров не регистрируются
	reviewUC  *usecase.ReviewUseCase  // nil → роуты отзывов не регистрируются
	stats     *usecase.StatsUseCase   // nil → роут статистики не регистрируется
	uploader  CoverUploader           // nil, если S3 не настроено
	log       *slog.Logger
}

func NewAdminHandler(uc *usecase.AdminUseCase, log *slog.Logger) *AdminHandler {
	return &AdminHandler{uc: uc, log: log}
}

// WithCoverUploader включает загрузку обложек категорий в S3. Если не вызван
// (или S3-ключи не заданы), эндпоинт загрузки вернёт 503.
func (h *AdminHandler) WithCoverUploader(up CoverUploader) *AdminHandler {
	h.uploader = up
	return h
}

// WithGenres включает CRUD справочника жанров и привязку жанров к категориям.
func (h *AdminHandler) WithGenres(uc *usecase.GenreUseCase) *AdminHandler {
	h.genreUC = uc
	return h
}

// WithExamples включает админский CRUD примеров готовых работ.
func (h *AdminHandler) WithExamples(uc *usecase.ExampleUseCase) *AdminHandler {
	h.exampleUC = uc
	return h
}

// WithReviews включает админскую модерацию отзывов (ответ, скрытие, удаление).
func (h *AdminHandler) WithReviews(uc *usecase.ReviewUseCase) *AdminHandler {
	h.reviewUC = uc
	return h
}

// WithStats включает эндпоинт сводной статистики для дашборда.
func (h *AdminHandler) WithStats(uc *usecase.StatsUseCase) *AdminHandler {
	h.stats = uc
	return h
}

func (h *AdminHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(RateLimiter(10, 20))

	r.Route("/accounts", func(r chi.Router) {
		r.Get("/", h.ListAccounts)
		r.Post("/", h.AddAccount)
		r.Patch("/{id}", h.SetAccountStatus)
		r.Post("/{id}/reset", h.ResetAccount)
	})

	r.Route("/orders", func(r chi.Router) {
		r.Get("/", h.ListOrders)
		r.Get("/{id}", h.GetOrder)
		r.Post("/{id}/refund", h.RefundOrder)
		r.Post("/{id}/confirm-payment", h.ConfirmOrderPayment)
		r.Post("/{id}/feedback", h.SendOrderFeedback)
		r.Post("/{id}/regenerate", h.RegenerateOrder)
		r.Delete("/{id}", h.DeleteOrder)
	})

	r.Route("/categories", func(r chi.Router) {
		r.Get("/", h.ListCategories)
		r.Get("/{id}", h.GetCategory)
		r.Post("/", h.CreateCategory)
		r.Put("/{id}", h.UpdateCategory)
		r.Post("/{id}/cover", h.UploadCover)
		r.Delete("/{id}", h.DeleteCategory)
		r.Post("/{id}/questions", h.AddQuestion)
		r.Put("/{id}/questions/{qid}", h.UpdateQuestion)
		r.Delete("/{id}/questions/{qid}", h.DeleteQuestion)
		if h.genreUC != nil {
			r.Get("/{id}/genres", h.GetCategoryGenres)
			r.Put("/{id}/genres", h.SetCategoryGenres)
		}
	})

	if h.genreUC != nil {
		r.Route("/genres", func(r chi.Router) {
			r.Get("/", h.ListGenres)
			r.Post("/", h.CreateGenre)
			r.Put("/{id}", h.UpdateGenre)
			r.Delete("/{id}", h.DeleteGenre)
		})
	}

	if h.exampleUC != nil {
		r.Route("/examples", func(r chi.Router) {
			r.Get("/", h.ListExamples)
			r.Post("/", h.CreateExample)
			r.Put("/{id}", h.UpdateExample)
			r.Delete("/{id}", h.DeleteExample)
			r.Post("/{id}/cover", h.UploadExampleCover)
		})
	}

	if h.reviewUC != nil {
		r.Route("/reviews", func(r chi.Router) {
			r.Get("/", h.ListReviews)
			r.Post("/{id}/reply", h.ReplyReview)
			r.Patch("/{id}", h.SetReviewPublished)
			r.Delete("/{id}", h.DeleteReview)
		})
	}

	if h.stats != nil {
		r.Get("/stats", h.GetStats)
	}

	return r
}

// --- DTOs ---

type addAccountRequest struct {
	Email         string `json:"email"`
	Session       string `json:"session"` // plaintext — сервер шифрует перед сохранением
	MaxConcurrent int    `json:"max_concurrent"`
}

type accountResponse struct {
	ID                 string `json:"id"`
	Email              string `json:"email"`
	Status             string `json:"status"`
	TokenBalance       int    `json:"token_balance"`
	MaxConcurrentTasks int    `json:"max_concurrent_tasks"`
	ConcurrentTasks    int    `json:"concurrent_tasks"`
	// CooldownUntil — момент окончания самовосстанавливающейся паузы (Throttle).
	// Пустая строка — паузы нет. Аккаунт при этом может иметь статус active, но
	// фактически выпадает из ротации до этого времени, поэтому показываем отдельно.
	CooldownUntil string `json:"cooldown_until,omitempty"`
	UpdatedAt     string `json:"updated_at"`
}

func accountToResponse(acc *domain.SunoAccount) accountResponse {
	resp := accountResponse{
		ID:                 acc.ID().String(),
		Email:              acc.Email(),
		Status:             string(acc.Status()),
		TokenBalance:       acc.TokenBalance(),
		MaxConcurrentTasks: acc.MaxConcurrentTasks(),
		ConcurrentTasks:    acc.ConcurrentTasks(),
		UpdatedAt:          acc.UpdatedAt().Format("2006-01-02T15:04:05Z"),
	}
	if cu := acc.CooldownUntil(); cu != nil {
		resp.CooldownUntil = cu.Format("2006-01-02T15:04:05Z")
	}
	return resp
}

type setStatusRequest struct {
	Status string `json:"status"`
}

type adminOrderResponse struct {
	ID                string          `json:"id"`
	InvoiceID         int64           `json:"invoice_id"`
	Email             string          `json:"email"`
	Phone             string          `json:"phone"`
	Brief             string          `json:"brief"`
	AmountKopecks     int64           `json:"amount_kopecks"`
	PaymentStatus     string          `json:"payment_status"`
	GenerationStatus  string          `json:"generation_status"`
	Tracks            []adminTrackDTO `json:"tracks"`
	AdminFeedback     string          `json:"admin_feedback,omitempty"`
	AdminFeedbackAt   string          `json:"admin_feedback_at,omitempty"`
	ConsentGivenAt    string          `json:"consent_given_at,omitempty"`
	ConsentDocVersion string          `json:"consent_doc_version,omitempty"`
	CreatedAt         string          `json:"created_at"`
}

type adminTrackDTO struct {
	Index    int    `json:"index"`
	AudioURL string `json:"audio_url"`
}

func orderToAdminResponse(o *domain.Order) adminOrderResponse {
	snap := o.Snapshot()
	tracks := make([]adminTrackDTO, 0, len(snap.Tracks))
	for _, t := range snap.Tracks {
		tracks = append(tracks, adminTrackDTO{Index: t.Index, AudioURL: t.AudioURL})
	}
	resp := adminOrderResponse{
		ID:                snap.ID.String(),
		InvoiceID:         snap.InvoiceID,
		Email:             snap.CustomerEmail,
		Phone:             snap.CustomerPhone,
		Brief:             snap.Brief,
		AmountKopecks:     snap.AmountKopecks,
		PaymentStatus:     string(snap.PaymentStatus),
		GenerationStatus:  string(snap.GenerationStatus),
		Tracks:            tracks,
		AdminFeedback:     snap.AdminFeedback,
		ConsentDocVersion: snap.ConsentDocVersion,
		CreatedAt:         snap.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if snap.ConsentGivenAt != nil {
		resp.ConsentGivenAt = snap.ConsentGivenAt.Format("2006-01-02T15:04:05Z")
	}
	if snap.AdminFeedbackAt != nil {
		resp.AdminFeedbackAt = snap.AdminFeedbackAt.Format("2006-01-02T15:04:05Z")
	}
	return resp
}

// --- Handlers ---

func (h *AdminHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.uc.ListAccounts(r.Context())
	if err != nil {
		h.log.Error("admin: ошибка получения аккаунтов", "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить список аккаунтов")
		return
	}

	resp := make([]accountResponse, 0, len(accounts))
	for _, acc := range accounts {
		resp = append(resp, accountToResponse(acc))
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *AdminHandler) AddAccount(w http.ResponseWriter, r *http.Request) {
	var req addAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	if req.Email == "" || req.Session == "" {
		respondError(w, r, http.StatusBadRequest, "email и session обязательны")
		return
	}

	acc, err := h.uc.AddAccount(r.Context(), req.Email, req.Session, req.MaxConcurrent)
	if err != nil {
		h.log.Error("admin: ошибка создания аккаунта", "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось создать аккаунт")
		return
	}
	respondJSON(w, http.StatusCreated, accountToResponse(acc))
}

func (h *AdminHandler) SetAccountStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID аккаунта")
		return
	}

	var req setStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	if err := h.uc.SetAccountStatus(r.Context(), id, domain.AccountStatus(req.Status)); err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			respondError(w, r, http.StatusNotFound, "аккаунт не найден")
			return
		}
		h.log.Error("admin: ошибка смены статуса аккаунта", "id", id, "error", err)
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ResetAccount полностью возвращает зависший аккаунт в пул: статус active,
// сброс счётчика ошибок и паузы, обнуление занятых слотов.
// POST /api/v1/admin/accounts/{id}/reset
func (h *AdminHandler) ResetAccount(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID аккаунта")
		return
	}

	if err := h.uc.ResetAccount(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			respondError(w, r, http.StatusNotFound, "аккаунт не найден")
			return
		}
		h.log.Error("admin: ошибка сброса аккаунта", "id", id, "error", err)
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) ListOrders(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	orders, total, err := h.uc.ListOrders(r.Context(), page, perPage)
	if err != nil {
		h.log.Error("admin: ошибка получения заказов", "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить список заказов")
		return
	}

	resp := make([]adminOrderResponse, 0, len(orders))
	for _, o := range orders {
		resp = append(resp, orderToAdminResponse(o))
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"orders": resp,
		"total":  total,
	})
}

func (h *AdminHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID заказа")
		return
	}

	order, err := h.uc.GetOrder(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
			return
		}
		h.log.Error("admin: ошибка получения заказа", "id", id, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить заказ")
		return
	}
	respondJSON(w, http.StatusOK, orderToAdminResponse(order))
}

// RefundOrder инициирует возврат платежа через Robokassa API.
// POST /api/v1/admin/orders/{id}/refund
func (h *AdminHandler) RefundOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID заказа")
		return
	}

	if err := h.uc.RefundOrder(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
			return
		}
		h.log.Error("admin: ошибка возврата платежа", "order_id", id, "error", err)
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ConfirmOrderPayment помечает заказ оплаченным, если ResultURL Robokassa не дошёл.
// POST /api/v1/admin/orders/{id}/confirm-payment
func (h *AdminHandler) ConfirmOrderPayment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID заказа")
		return
	}

	if err := h.uc.ConfirmOrderPayment(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
			return
		}
		h.log.Error("admin: ошибка подтверждения оплаты", "order_id", id, "error", err)
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RegenerateOrder повторно ставит оплаченный, но упавший заказ в очередь генерации.
// POST /api/v1/admin/orders/{id}/regenerate
func (h *AdminHandler) RegenerateOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID заказа")
		return
	}

	if err := h.uc.RegenerateOrder(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
			return
		}
		h.log.Error("admin: ошибка перегенерации заказа", "order_id", id, "error", err)
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteOrder безвозвратно удаляет заказ и MP3-треки в хранилище.
// DELETE /api/v1/admin/orders/{id}
func (h *AdminHandler) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID заказа")
		return
	}

	if err := h.uc.DeleteOrder(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
			return
		}
		h.log.Error("admin: ошибка удаления заказа", "order_id", id, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось удалить заказ")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type orderFeedbackRequest struct {
	Message string `json:"message"`
}

// SendOrderFeedback фиксирует сообщение администратора по заказу и отправляет
// его клиенту на email.
// POST /api/v1/admin/orders/{id}/feedback
func (h *AdminHandler) SendOrderFeedback(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный UUID заказа")
		return
	}

	var req orderFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	if err := h.uc.SendOrderFeedback(r.Context(), id, req.Message); err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			respondError(w, r, http.StatusNotFound, "заказ не найден")
			return
		}
		h.log.Error("admin: ошибка отправки обратной связи", "order_id", id, "error", err)
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ==========================================
// Категории и вопросы квиза (server-driven UI)
// ==========================================

type categoryRequest struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	CoverImageURL      string   `json:"cover_image_url"`
	SeoTags            []string `json:"seo_tags"`
	BasePromptTemplate string   `json:"base_prompt_template"`
}

type optionDTO struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type questionRequest struct {
	StepNumber   int                   `json:"step_number"`
	QuestionText string                `json:"question_text"`
	UIType       string                `json:"ui_type"`
	MappingKey   string                `json:"mapping_key"`
	IsRequired   bool                  `json:"is_required"`
	OptionSource string                `json:"option_source"`
	Config       domain.QuestionConfig `json:"config"`
	Options      []optionDTO           `json:"options"`
}

func (q questionRequest) toDomainConfig() domain.QuestionConfig {
	return q.Config
}

func (q questionRequest) toDomainOptions() []domain.Option {
	opts := make([]domain.Option, 0, len(q.Options))
	for _, o := range q.Options {
		opts = append(opts, domain.Option{Label: o.Label, Value: o.Value})
	}
	return opts
}

type categoryAdminResponse struct {
	ID                 string             `json:"id"`
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	CoverImageURL      string             `json:"cover_image_url"`
	SeoTags            []string           `json:"seo_tags"`
	BasePromptTemplate string             `json:"base_prompt_template"`
	GenreIDs           []int              `json:"genre_ids,omitempty"`
	Questions          []questionResponse `json:"questions,omitempty"`
}

type questionResponse struct {
	ID           int                   `json:"id"`
	StepNumber   int                   `json:"step_number"`
	QuestionText string                `json:"question_text"`
	UIType       string                `json:"ui_type"`
	MappingKey   string                `json:"mapping_key"`
	IsRequired   bool                  `json:"is_required"`
	OptionSource string                `json:"option_source,omitempty"`
	Config       domain.QuestionConfig `json:"config,omitempty"`
	Options      []optionDTO           `json:"options,omitempty"`
}

func questionToResponse(q domain.Question) questionResponse {
	opts := make([]optionDTO, 0, len(q.Options))
	for _, o := range q.Options {
		opts = append(opts, optionDTO{Label: o.Label, Value: o.Value})
	}
	return questionResponse{
		ID:           q.ID,
		StepNumber:   q.StepNumber,
		QuestionText: q.QuestionText,
		UIType:       q.UIType,
		MappingKey:   q.MappingKey,
		IsRequired:   q.IsRequired,
		OptionSource: q.OptionSource,
		Config:       q.Config,
		Options:      opts,
	}
}

func categoryToAdminResponse(c *domain.Category, genreIDs []int) categoryAdminResponse {
	questions := make([]questionResponse, 0, len(c.Questions()))
	for _, q := range c.Questions() {
		questions = append(questions, questionToResponse(q))
	}
	return categoryAdminResponse{
		ID:                 c.ID(),
		Title:              c.Title(),
		Description:        c.Description(),
		CoverImageURL:      c.CoverImageURL(),
		SeoTags:            c.SeoTags(),
		BasePromptTemplate: c.BasePromptTemplate(),
		GenreIDs:           genreIDs,
		Questions:          questions,
	}
}

// ListCategories возвращает все категории (без вопросов) для списка в админке.
// GET /api/v1/admin/categories
func (h *AdminHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.uc.ListCategories(r.Context())
	if err != nil {
		h.log.Error("admin: ошибка получения категорий", "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить список категорий")
		return
	}
	resp := make([]categoryAdminResponse, 0, len(categories))
	for _, c := range categories {
		resp = append(resp, categoryToAdminResponse(c, nil))
	}
	respondJSON(w, http.StatusOK, resp)
}

// GetCategory возвращает категорию со всеми вопросами для формы редактирования.
// GET /api/v1/admin/categories/{id}
func (h *AdminHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	category, err := h.uc.GetCategory(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrCategoryNotFound) {
			respondError(w, r, http.StatusNotFound, "категория не найдена")
			return
		}
		h.log.Error("admin: ошибка получения категории", "category_id", id, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить категорию")
		return
	}
	var genreIDs []int
	if h.genreUC != nil {
		genreIDs, err = h.genreUC.GetCategoryGenreIDs(r.Context(), id)
		if err != nil {
			h.log.Error("admin: ошибка получения жанров категории", "category_id", id, "error", err)
			respondError(w, r, http.StatusInternalServerError, "не удалось получить категорию")
			return
		}
	}
	respondJSON(w, http.StatusOK, categoryToAdminResponse(category, genreIDs))
}

// CreateCategory создаёт новую категорию каталога (включая, например, категорию
// "general"/"freeform" для свободного сценария без жёсткого набора вопросов).
// POST /api/v1/admin/categories
func (h *AdminHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	category, err := h.uc.CreateCategory(r.Context(), req.ID, req.Title, req.Description, req.CoverImageURL, req.SeoTags, req.BasePromptTemplate)
	if err != nil {
		if errors.Is(err, domain.ErrCategoryAlreadyExists) {
			respondError(w, r, http.StatusConflict, "категория с таким id уже существует")
			return
		}
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, categoryToAdminResponse(category, nil))
}

// UpdateCategory обновляет изменяемые поля категории.
// PUT /api/v1/admin/categories/{id}
func (h *AdminHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req categoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	category, err := h.uc.UpdateCategory(r.Context(), id, req.Title, req.Description, req.CoverImageURL, req.SeoTags, req.BasePromptTemplate)
	if err != nil {
		if errors.Is(err, domain.ErrCategoryNotFound) {
			respondError(w, r, http.StatusNotFound, "категория не найдена")
			return
		}
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, categoryToAdminResponse(category, nil))
}

// coverExtByMIME — допустимые типы обложек и соответствующее расширение ключа.
var coverExtByMIME = map[string]string{
	"image/webp": "webp",
	"image/png":  "png",
	"image/jpeg": "jpg",
}

// UploadCover принимает файл обложки (multipart, поле "file"), загружает его в
// S3 и возвращает постоянную публичную ссылку. Сам URL в категорию не
// записывает — это делает последующий PUT категории с фронтенда.
// POST /api/v1/admin/categories/{id}/cover
func (h *AdminHandler) UploadCover(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if h.uploader == nil {
		respondError(w, r, http.StatusServiceUnavailable, "загрузка обложек недоступна: не настроено S3-хранилище (S3_ACCESS_KEY/S3_SECRET_KEY)")
		return
	}

	// Глобальный лимит тела запроса — 1 МБ; держимся в его пределах.
	const maxCoverBytes = 1 << 20
	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "файл не передан (ожидается поле формы \"file\")")
		return
	}
	defer file.Close() //nolint:errcheck

	data, err := io.ReadAll(io.LimitReader(file, maxCoverBytes+1))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "не удалось прочитать файл")
		return
	}
	if len(data) == 0 {
		respondError(w, r, http.StatusBadRequest, "файл пустой")
		return
	}
	if len(data) > maxCoverBytes {
		respondError(w, r, http.StatusRequestEntityTooLarge, "файл слишком большой (максимум 1 МБ)")
		return
	}

	contentType := http.DetectContentType(data)
	ext, ok := coverExtByMIME[contentType]
	if !ok {
		respondError(w, r, http.StatusUnsupportedMediaType, "поддерживаются только изображения PNG, JPEG и WebP")
		return
	}

	// Уникальный ключ с меткой времени — старая обложка не перетирается, а смена
	// URL гарантированно сбрасывает кэш браузера/CDN.
	key := fmt.Sprintf("covers/%s-%d.%s", id, time.Now().Unix(), ext)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	url, err := h.uploader.Upload(ctx, key, contentType, data)
	if err != nil {
		h.log.Error("admin: ошибка загрузки обложки", "category_id", id, "error", err)
		respondError(w, r, http.StatusBadGateway, "не удалось загрузить обложку в хранилище")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"cover_image_url": url})
}

// DeleteCategory удаляет категорию вместе со всеми её вопросами.
// DELETE /api/v1/admin/categories/{id}
func (h *AdminHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.uc.DeleteCategory(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrCategoryNotFound) {
			respondError(w, r, http.StatusNotFound, "категория не найдена")
			return
		}
		h.log.Error("admin: ошибка удаления категории", "category_id", id, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось удалить категорию")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// AddQuestion добавляет вопрос квиза к категории.
// POST /api/v1/admin/categories/{id}/questions
func (h *AdminHandler) AddQuestion(w http.ResponseWriter, r *http.Request) {
	categoryID := chi.URLParam(r, "id")

	var req questionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	q, err := h.uc.AddQuestion(r.Context(), categoryID, req.StepNumber, req.QuestionText, req.UIType, req.MappingKey, req.IsRequired, req.OptionSource, req.toDomainConfig(), req.toDomainOptions())
	if err != nil {
		if errors.Is(err, domain.ErrCategoryNotFound) {
			respondError(w, r, http.StatusNotFound, "категория не найдена")
			return
		}
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, questionToResponse(q))
}

// UpdateQuestion перезаписывает вопрос квиза (включая полную замену вариантов ответов).
// PUT /api/v1/admin/categories/{id}/questions/{qid}
func (h *AdminHandler) UpdateQuestion(w http.ResponseWriter, r *http.Request) {
	categoryID := chi.URLParam(r, "id")
	qid, err := strconv.Atoi(chi.URLParam(r, "qid"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный id вопроса")
		return
	}

	var req questionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	if err := h.uc.UpdateQuestion(r.Context(), categoryID, qid, req.StepNumber, req.QuestionText, req.UIType, req.MappingKey, req.IsRequired, req.OptionSource, req.toDomainConfig(), req.toDomainOptions()); err != nil {
		if errors.Is(err, domain.ErrQuestionNotFound) {
			respondError(w, r, http.StatusNotFound, "вопрос не найден")
			return
		}
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteQuestion удаляет вопрос квиза.
// DELETE /api/v1/admin/categories/{id}/questions/{qid}
func (h *AdminHandler) DeleteQuestion(w http.ResponseWriter, r *http.Request) {
	categoryID := chi.URLParam(r, "id")
	qid, err := strconv.Atoi(chi.URLParam(r, "qid"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный id вопроса")
		return
	}

	if err := h.uc.DeleteQuestion(r.Context(), categoryID, qid); err != nil {
		if errors.Is(err, domain.ErrQuestionNotFound) {
			respondError(w, r, http.StatusNotFound, "вопрос не найден")
			return
		}
		h.log.Error("admin: ошибка удаления вопроса", "category_id", categoryID, "question_id", qid, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось удалить вопрос")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Genres ---

type genreRequest struct {
	Slug      string `json:"slug"`
	Label     string `json:"label"`
	SunoValue string `json:"suno_value"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

type genreUpdateRequest struct {
	Label     string `json:"label"`
	SunoValue string `json:"suno_value"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

type categoryGenresRequest struct {
	GenreIDs []int `json:"genre_ids"`
}

// ListGenres возвращает весь справочник жанров (включая неактивные).
// GET /api/v1/admin/genres
func (h *AdminHandler) ListGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := h.genreUC.List(r.Context(), "", false)
	if err != nil {
		h.log.Error("admin: ошибка получения жанров", "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить жанры")
		return
	}
	respondJSON(w, http.StatusOK, genres)
}

// CreateGenre создаёт жанр в справочнике.
// POST /api/v1/admin/genres
func (h *AdminHandler) CreateGenre(w http.ResponseWriter, r *http.Request) {
	var req genreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	g, err := h.genreUC.Create(r.Context(), req.Slug, req.Label, req.SunoValue, req.SortOrder)
	if err != nil {
		if errors.Is(err, domain.ErrGenreAlreadyExists) {
			respondError(w, r, http.StatusConflict, "жанр с таким slug уже существует")
			return
		}
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, g)
}

// UpdateGenre обновляет жанр.
// PUT /api/v1/admin/genres/{id}
func (h *AdminHandler) UpdateGenre(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный id жанра")
		return
	}
	var req genreUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	g, err := h.genreUC.Update(r.Context(), id, req.Label, req.SunoValue, req.SortOrder, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrGenreNotFound) {
			respondError(w, r, http.StatusNotFound, "жанр не найден")
			return
		}
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, g)
}

// DeleteGenre удаляет жанр из справочника.
// DELETE /api/v1/admin/genres/{id}
func (h *AdminHandler) DeleteGenre(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "некорректный id жанра")
		return
	}
	if err := h.genreUC.Delete(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrGenreNotFound) {
			respondError(w, r, http.StatusNotFound, "жанр не найден")
			return
		}
		h.log.Error("admin: ошибка удаления жанра", "genre_id", id, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось удалить жанр")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetCategoryGenres возвращает id жанров, привязанных к категории.
// GET /api/v1/admin/categories/{id}/genres
func (h *AdminHandler) GetCategoryGenres(w http.ResponseWriter, r *http.Request) {
	categoryID := chi.URLParam(r, "id")
	ids, err := h.genreUC.GetCategoryGenreIDs(r.Context(), categoryID)
	if err != nil {
		h.log.Error("admin: ошибка получения жанров категории", "category_id", categoryID, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить жанры категории")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"genre_ids": ids})
}

// SetCategoryGenres заменяет список жанров категории.
// PUT /api/v1/admin/categories/{id}/genres
func (h *AdminHandler) SetCategoryGenres(w http.ResponseWriter, r *http.Request) {
	categoryID := chi.URLParam(r, "id")
	var req categoryGenresRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	if err := h.genreUC.SetCategoryGenres(r.Context(), categoryID, req.GenreIDs); err != nil {
		h.log.Error("admin: ошибка обновления жанров категории", "category_id", categoryID, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось обновить жанры категории")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
