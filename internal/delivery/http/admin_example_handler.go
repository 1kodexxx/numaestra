package apphttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/numaestra/numaestra/internal/domain"
)

type exampleRequest struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Mood        string `json:"mood"`
	AudioURL    string `json:"audio_url"`
	CoverURL    string `json:"cover_url"`
	SortOrder   int    `json:"sort_order"`
	IsActive    bool   `json:"is_active"`
}

type exampleAdminResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Mood        string `json:"mood"`
	AudioURL    string `json:"audio_url"`
	CoverURL    string `json:"cover_url"`
	SortOrder   int    `json:"sort_order"`
	IsActive    bool   `json:"is_active"`
}

func exampleToAdminResponse(e *domain.Example) exampleAdminResponse {
	return exampleAdminResponse{
		ID:          e.ID(),
		Title:       e.Title(),
		Category:    e.Category(),
		Description: e.Description(),
		Mood:        e.Mood(),
		AudioURL:    e.AudioURL(),
		CoverURL:    e.CoverURL(),
		SortOrder:   e.SortOrder(),
		IsActive:    e.IsActive(),
	}
}

// ListExamples возвращает все примеры (включая скрытые) для админки.
// GET /api/v1/admin/examples
func (h *AdminHandler) ListExamples(w http.ResponseWriter, r *http.Request) {
	examples, err := h.exampleUC.List(r.Context())
	if err != nil {
		h.log.Error("admin: ошибка получения примеров", "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить список примеров")
		return
	}
	resp := make([]exampleAdminResponse, 0, len(examples))
	for _, e := range examples {
		resp = append(resp, exampleToAdminResponse(e))
	}
	respondJSON(w, http.StatusOK, resp)
}

// CreateExample создаёт новый пример.
// POST /api/v1/admin/examples
func (h *AdminHandler) CreateExample(w http.ResponseWriter, r *http.Request) {
	var req exampleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	e, err := h.exampleUC.Create(r.Context(), req.ID, req.Title, req.Category, req.Description, req.Mood, req.AudioURL, req.CoverURL, req.SortOrder, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrExampleAlreadyExists) {
			respondError(w, r, http.StatusConflict, "пример с таким id уже существует")
			return
		}
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, exampleToAdminResponse(e))
}

// UpdateExample обновляет изменяемые поля примера.
// PUT /api/v1/admin/examples/{id}
func (h *AdminHandler) UpdateExample(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req exampleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, r, http.StatusBadRequest, "неверный формат JSON")
		return
	}
	e, err := h.exampleUC.Update(r.Context(), id, req.Title, req.Category, req.Description, req.Mood, req.AudioURL, req.CoverURL, req.SortOrder, req.IsActive)
	if err != nil {
		if errors.Is(err, domain.ErrExampleNotFound) {
			respondError(w, r, http.StatusNotFound, "пример не найден")
			return
		}
		respondError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, exampleToAdminResponse(e))
}

// DeleteExample удаляет пример.
// DELETE /api/v1/admin/examples/{id}
func (h *AdminHandler) DeleteExample(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.exampleUC.Delete(r.Context(), id); err != nil {
		if errors.Is(err, domain.ErrExampleNotFound) {
			respondError(w, r, http.StatusNotFound, "пример не найден")
			return
		}
		h.log.Error("admin: ошибка удаления примера", "example_id", id, "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось удалить пример")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UploadExampleCover загружает обложку примера в S3 и возвращает публичную ссылку.
// POST /api/v1/admin/examples/{id}/cover
func (h *AdminHandler) UploadExampleCover(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.uploader == nil {
		respondError(w, r, http.StatusServiceUnavailable, "загрузка обложек недоступна: не настроено S3-хранилище (S3_ACCESS_KEY/S3_SECRET_KEY)")
		return
	}

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

	key := fmt.Sprintf("examples/%s-%d.%s", id, time.Now().Unix(), ext)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	url, err := h.uploader.Upload(ctx, key, contentType, data)
	if err != nil {
		h.log.Error("admin: ошибка загрузки обложки примера", "example_id", id, "error", err)
		respondError(w, r, http.StatusBadGateway, "не удалось загрузить обложку в хранилище")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"cover_url": url})
}

// GetStats отдаёт сводную статистику для дашборда.
// GET /api/v1/admin/stats
func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	s, err := h.stats.GetStats(r.Context())
	if err != nil {
		h.log.Error("admin: ошибка получения статистики", "error", err)
		respondError(w, r, http.StatusInternalServerError, "не удалось получить статистику")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"orders": map[string]any{
			"total":           s.Orders.TotalOrders,
			"paid":            s.Orders.PaidOrders,
			"revenue_kopecks": s.Orders.RevenueKopecks,
			"completed":       s.Orders.Completed,
			"processing":      s.Orders.Processing,
			"failed":          s.Orders.Failed,
			"today":           s.Orders.OrdersToday,
		},
		"accounts": map[string]any{
			"total":         s.AccountsTotal,
			"active":        s.AccountsActive,
			"token_balance": s.TokenBalance,
		},
		"categories_total": s.CategoriesTotal,
		"examples_total":   s.ExamplesTotal,
		"examples_active":  s.ExamplesActive,
	})
}
