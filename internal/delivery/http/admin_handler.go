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
)

// AdminHandler обрабатывает запросы административного API.
// Все маршруты защищены Bearer-токеном через AdminAuth middleware.
type AdminHandler struct {
	uc  *usecase.AdminUseCase
	log *slog.Logger
}

func NewAdminHandler(uc *usecase.AdminUseCase, log *slog.Logger) *AdminHandler {
	return &AdminHandler{uc: uc, log: log}
}

func (h *AdminHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(RateLimiter(10, 20))

	r.Route("/accounts", func(r chi.Router) {
		r.Get("/", h.ListAccounts)
		r.Post("/", h.AddAccount)
		r.Patch("/{id}", h.SetAccountStatus)
	})

	r.Route("/orders", func(r chi.Router) {
		r.Get("/", h.ListOrders)
		r.Get("/{id}", h.GetOrder)
		r.Post("/{id}/refund", h.RefundOrder)
	})

	r.Route("/categories", func(r chi.Router) {
		r.Post("/", h.CreateCategory)
		r.Put("/{id}", h.UpdateCategory)
		r.Delete("/{id}", h.DeleteCategory)
		r.Post("/{id}/questions", h.AddQuestion)
		r.Put("/{id}/questions/{qid}", h.UpdateQuestion)
		r.Delete("/{id}/questions/{qid}", h.DeleteQuestion)
	})

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
	UpdatedAt          string `json:"updated_at"`
}

func accountToResponse(acc *domain.SunoAccount) accountResponse {
	return accountResponse{
		ID:                 acc.ID().String(),
		Email:              acc.Email(),
		Status:             string(acc.Status()),
		TokenBalance:       acc.TokenBalance(),
		MaxConcurrentTasks: acc.MaxConcurrentTasks(),
		ConcurrentTasks:    acc.ConcurrentTasks(),
		UpdatedAt:          acc.UpdatedAt().Format("2006-01-02T15:04:05Z"),
	}
}

type setStatusRequest struct {
	Status string `json:"status"`
}

type adminOrderResponse struct {
	ID               string          `json:"id"`
	InvoiceID        int64           `json:"invoice_id"`
	Email            string          `json:"email"`
	Phone            string          `json:"phone"`
	Brief            string          `json:"brief"`
	AmountKopecks    int64           `json:"amount_kopecks"`
	PaymentStatus    string          `json:"payment_status"`
	GenerationStatus string          `json:"generation_status"`
	Tracks           []adminTrackDTO `json:"tracks"`
	CreatedAt        string          `json:"created_at"`
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
	return adminOrderResponse{
		ID:               snap.ID.String(),
		InvoiceID:        snap.InvoiceID,
		Email:            snap.CustomerEmail,
		Phone:            snap.CustomerPhone,
		Brief:            snap.Brief,
		AmountKopecks:    snap.AmountKopecks,
		PaymentStatus:    string(snap.PaymentStatus),
		GenerationStatus: string(snap.GenerationStatus),
		Tracks:           tracks,
		CreatedAt:        snap.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
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
	StepNumber   int         `json:"step_number"`
	QuestionText string      `json:"question_text"`
	UIType       string      `json:"ui_type"`
	MappingKey   string      `json:"mapping_key"`
	IsRequired   bool        `json:"is_required"`
	Options      []optionDTO `json:"options"`
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
	Questions          []questionResponse `json:"questions,omitempty"`
}

type questionResponse struct {
	ID           int         `json:"id"`
	StepNumber   int         `json:"step_number"`
	QuestionText string      `json:"question_text"`
	UIType       string      `json:"ui_type"`
	MappingKey   string      `json:"mapping_key"`
	IsRequired   bool        `json:"is_required"`
	Options      []optionDTO `json:"options,omitempty"`
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
		Options:      opts,
	}
}

func categoryToAdminResponse(c *domain.Category) categoryAdminResponse {
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
		Questions:          questions,
	}
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
	respondJSON(w, http.StatusCreated, categoryToAdminResponse(category))
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
	respondJSON(w, http.StatusOK, categoryToAdminResponse(category))
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

	q, err := h.uc.AddQuestion(r.Context(), categoryID, req.StepNumber, req.QuestionText, req.UIType, req.MappingKey, req.IsRequired, req.toDomainOptions())
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

	if err := h.uc.UpdateQuestion(r.Context(), categoryID, qid, req.StepNumber, req.QuestionText, req.UIType, req.MappingKey, req.IsRequired, req.toDomainOptions()); err != nil {
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
