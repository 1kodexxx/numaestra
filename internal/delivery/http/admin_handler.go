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

	return r
}

// --- DTOs ---

type addAccountRequest struct {
	Email            string `json:"email"`
	EncryptedSession string `json:"encrypted_session"`
	MaxConcurrent    int    `json:"max_concurrent"`
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
	Plan             string          `json:"plan,omitempty"`
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
		h.errJSON(w, http.StatusInternalServerError, "не удалось получить список аккаунтов")
		return
	}

	resp := make([]accountResponse, 0, len(accounts))
	for _, acc := range accounts {
		resp = append(resp, accountToResponse(acc))
	}
	h.jsonResponse(w, http.StatusOK, resp)
}

func (h *AdminHandler) AddAccount(w http.ResponseWriter, r *http.Request) {
	var req addAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errJSON(w, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	if req.Email == "" || req.EncryptedSession == "" {
		h.errJSON(w, http.StatusBadRequest, "email и encrypted_session обязательны")
		return
	}

	acc, err := h.uc.AddAccount(r.Context(), req.Email, req.EncryptedSession, req.MaxConcurrent)
	if err != nil {
		h.log.Error("admin: ошибка создания аккаунта", "error", err)
		h.errJSON(w, http.StatusInternalServerError, "не удалось создать аккаунт")
		return
	}
	h.jsonResponse(w, http.StatusCreated, accountToResponse(acc))
}

func (h *AdminHandler) SetAccountStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.errJSON(w, http.StatusBadRequest, "некорректный UUID аккаунта")
		return
	}

	var req setStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errJSON(w, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	if err := h.uc.SetAccountStatus(r.Context(), id, domain.AccountStatus(req.Status)); err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			h.errJSON(w, http.StatusNotFound, "аккаунт не найден")
			return
		}
		h.log.Error("admin: ошибка смены статуса аккаунта", "id", id, "error", err)
		h.errJSON(w, http.StatusBadRequest, err.Error())
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
		h.errJSON(w, http.StatusInternalServerError, "не удалось получить список заказов")
		return
	}

	resp := make([]adminOrderResponse, 0, len(orders))
	for _, o := range orders {
		resp = append(resp, orderToAdminResponse(o))
	}
	h.jsonResponse(w, http.StatusOK, map[string]any{
		"orders": resp,
		"total":  total,
	})
}

func (h *AdminHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.errJSON(w, http.StatusBadRequest, "некорректный UUID заказа")
		return
	}

	order, err := h.uc.GetOrder(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			h.errJSON(w, http.StatusNotFound, "заказ не найден")
			return
		}
		h.log.Error("admin: ошибка получения заказа", "id", id, "error", err)
		h.errJSON(w, http.StatusInternalServerError, "не удалось получить заказ")
		return
	}
	h.jsonResponse(w, http.StatusOK, orderToAdminResponse(order))
}

// RefundOrder инициирует возврат платежа через Robokassa API.
// POST /api/v1/admin/orders/{id}/refund
func (h *AdminHandler) RefundOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.errJSON(w, http.StatusBadRequest, "некорректный UUID заказа")
		return
	}

	if err := h.uc.RefundOrder(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrOrderNotFound) {
			h.errJSON(w, http.StatusNotFound, "заказ не найден")
			return
		}
		h.log.Error("admin: ошибка возврата платежа", "order_id", id, "error", err)
		h.errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) jsonResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

func (h *AdminHandler) errJSON(w http.ResponseWriter, status int, msg string) {
	h.jsonResponse(w, status, map[string]string{"error": msg})
}
