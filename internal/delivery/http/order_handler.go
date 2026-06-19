// internal/delivery/http/order_handler.go
package http

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"errors"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/numaestra/numaestra/internal/config"
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/internal/usecase"
)

// OrderHandler обрабатывает HTTP-запросы, связанные с заказами.
type OrderHandler struct {
	uc  *usecase.OrderUseCase
	log *slog.Logger
	rk  config.RobokassaConfig
}

// Теперь передаем весь конфиг Робокассы
func NewOrderHandler(uc *usecase.OrderUseCase, log *slog.Logger, rk config.RobokassaConfig) *OrderHandler {
	return &OrderHandler{
		uc:  uc,
		log: log,
		rk:  rk,
	}
}

func (h *OrderHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/", h.CreateOrder)
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

	// 1. ГЕНЕРАЦИЯ ССЫЛКИ НА ОПЛАТУ
	// Робокасса принимает сумму в рублях с двумя знаками после запятой ("1500.00").
	// Целочисленное деление kopecks/100 обрезает копейки и ломает подпись —
	// Robokassa сверяет SignatureValue именно по той строке OutSum, что пришла в вебхуке.
	outSum := fmt.Sprintf("%.2f", float64(req.AmountKopecks)/100)
	invIdStr := fmt.Sprintf("%d", order.InvoiceID())
	description := "Генерация 4-х версий студийной песни Numaestra"

	// Формула подписи для генерации ссылки: MerchantLogin:OutSum:InvId:Password1
	signStr := fmt.Sprintf("%s:%s:%s:%s", h.rk.MerchantLogin, outSum, invIdStr, h.rk.Password1)
	hash := md5.Sum([]byte(signStr))
	signature := strings.ToUpper(hex.EncodeToString(hash[:]))

	isTest := "0"
	if h.rk.IsTest {
		isTest = "1"
	}

	paymentURL := fmt.Sprintf("https://auth.robokassa.ru/Merchant/Index.aspx?MerchantLogin=%s&OutSum=%s&InvId=%s&Description=%s&SignatureValue=%s&IsTest=%s",
		h.rk.MerchantLogin, outSum, invIdStr, url.QueryEscape(description), signature, isTest)

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

	// 2. ВАЛИДАЦИЯ ВЕБХУКА ОТ КАССЫ
	// Формула подписи ResultURL: OutSum:InvId:Password2
	signStr := fmt.Sprintf("%s:%s:%s", outSum, invIdStr, h.rk.Password2)
	hash := md5.Sum([]byte(signStr))
	expectedSignature := strings.ToUpper(hex.EncodeToString(hash[:]))

	if strings.ToUpper(signature) != expectedSignature {
		h.log.Warn("попытка подделки вебхука Робокассы!", "expected", expectedSignature, "got", signature)
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
