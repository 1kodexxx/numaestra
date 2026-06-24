// pkg/suno/client.go
//
// Клиент TTAPI для генерации музыки через Suno (https://docs.ttapi.io/api/en/suno).
// Модель асинхронная: создаём задачу (POST /suno/v1/music) → получаем jobId →
// опрашиваем (GET /suno/v2/fetch?jobId=...), пока статус не станет терминальным.
// Авторизация — заголовок TT-API-KEY.
package suno

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Терминальные и промежуточные статусы задачи (нормализованные из TTAPI).
const (
	StatusPending = "pending" // принята, ждёт обработки
	StatusRunning = "running" // обрабатывается провайдером
	StatusSuccess = "success" // готово, Clips заполнены
	StatusFailure = "failure" // ошибка; кредиты возвращены
	StatusTimeout = "timeout" // таймаут; кредиты возвращены
)

// Идентификаторы моделей Suno в TTAPI (параметр mv).
// Документация: https://docs.ttapi.io/api/en/suno/suno_music
const (
	ModelV5      = "chirp-v5"   // Suno v5
	ModelV55     = "chirp-v5-5" // Suno v5.5 — текущая топовая модель TTAPI
	DefaultModel = ModelV55     // модель по умолчанию для продакшена
)

// Доменно-значимые ошибки API.
var (
	// ErrInvalidAPIKey — 401: ключ отсутствует или недействителен.
	ErrInvalidAPIKey = errors.New("suno: неверный или отсутствующий API-ключ (401)")
	// ErrInsufficientCredits — 402: на балансе недостаточно кредитов.
	ErrInsufficientCredits = errors.New("suno: недостаточно кредитов на балансе (402)")
	// ErrRateLimited — 429: превышен лимит запросов.
	ErrRateLimited = errors.New("suno: превышен лимит запросов (429)")
	// ErrTaskNotFound — 404: задача не найдена.
	ErrTaskNotFound = errors.New("suno: задача не найдена (404)")
)

// APIClient — порт клиента для адаптера провайдера.
type APIClient interface {
	// CreateMusicTask создаёт задачу генерации музыки и возвращает jobId.
	CreateMusicTask(ctx context.Context, in MusicInput) (taskID string, err error)
	// GetTask возвращает текущее состояние задачи: статус и (по готовности) клипы.
	GetTask(ctx context.Context, taskID string) (Task, error)
}

// MusicInput — параметры генерации.
// При Instrumental=false используется Inspiration Mode TTAPI (custom=false):
// Description — свободный текст, AI сам пишет текст и музыку.
type MusicInput struct {
	Description  string // prompt — описание/бриф будущей песни
	Tags         string // tags — стиль/жанр через запятую (опционально)
	Instrumental bool   // instrumental — песня без вокала
}

// Task — провайдеро-нейтральное представление состояния задачи.
type Task struct {
	ID         string
	Status     string // одна из StatusPending/Running/Success/Failure/Timeout
	Clips      []Clip // заполнено при StatusSuccess
	FailReason string // причина при StatusFailure/StatusTimeout
}

// Clip — один сгенерированный трек.
type Clip struct {
	ID          string
	AudioURL    string
	DurationSec int
}

type httpClient struct {
	baseURL string
	apiKey  string
	mv      string // модель Suno (mv), например chirp-v5-5
	client  *http.Client
}

// NewClient создаёт клиент TTAPI. baseURL — корень API (например, https://api.ttapi.io);
// apiKey уходит в заголовок TT-API-KEY. model — параметр mv (пустая строка → DefaultModel).
func NewClient(baseURL, apiKey, model string) APIClient {
	mv := model
	if mv == "" {
		mv = DefaultModel
	}
	return &httpClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		mv:      mv,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- DTO внешнего API TTAPI (camelCase согласно docs.ttapi.io) ---

type createMusicRequest struct {
	Custom bool   `json:"custom"`
	Mv     string `json:"mv"`
	// Inspiration Mode (custom=false): gpt_description_prompt — описание для AI.
	// Custom Mode (custom=true): prompt — готовые тексты песни.
	GptDescriptionPrompt string `json:"gpt_description_prompt,omitempty"`
	Prompt               string `json:"prompt,omitempty"`
	Instrumental         bool   `json:"instrumental"`
	Tags                 string `json:"tags,omitempty"`
}

type createMusicResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		JobID string `json:"jobId"`
	} `json:"data"`
}

type fetchResultResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		JobID  string        `json:"jobId"`
		Musics []musicResult `json:"musics"`
	} `json:"data"`
}

type musicResult struct {
	MusicID  string `json:"musicId"`
	AudioURL string `json:"audioUrl"`
	// Duration приходит как строка ("11.44"), а не число — особенность TTAPI.
	Duration json.Number `json:"duration"`
}

// CreateMusicTask отправляет POST /suno/v1/music в Inspiration Mode (custom=false):
// AI самостоятельно пишет слова и музыку по описанию в поле prompt.
func (c *httpClient) CreateMusicTask(ctx context.Context, in MusicInput) (string, error) {
	body, err := json.Marshal(createMusicRequest{
		Custom:               false, // Inspiration Mode: AI генерирует текст из описания
		Mv:                   c.mv,
		GptDescriptionPrompt: in.Description,
		Instrumental:         in.Instrumental,
		Tags:                 in.Tags,
	})
	if err != nil {
		return "", fmt.Errorf("marshal create task: %w", err)
	}

	raw, err := c.do(ctx, http.MethodPost, "/suno/v1/music", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	var resp createMusicResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("decode create task: %w", err)
	}
	if resp.Data.JobID == "" {
		return "", fmt.Errorf("suno: пустой jobId в ответе: %s", string(raw))
	}
	return resp.Data.JobID, nil
}

// GetTask опрашивает GET /suno/v2/fetch?jobId={taskID}.
// TTAPI статусы: ON_QUEUE → running, SUCCESS → success, FAILED → failure.
func (c *httpClient) GetTask(ctx context.Context, taskID string) (Task, error) {
	if taskID == "" {
		return Task{}, fmt.Errorf("suno: пустой task id")
	}

	raw, err := c.do(ctx, http.MethodGet, "/suno/v2/fetch?jobId="+url.QueryEscape(taskID), nil)
	if err != nil {
		return Task{}, err
	}

	var resp fetchResultResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Task{}, fmt.Errorf("decode get task: %w", err)
	}

	task := Task{ID: taskID}
	switch resp.Status {
	case "SUCCESS":
		task.Status = StatusSuccess
		for _, m := range resp.Data.Musics {
			dur, _ := strconv.ParseFloat(m.Duration.String(), 64)
			task.Clips = append(task.Clips, Clip{
				ID:          m.MusicID,
				AudioURL:    m.AudioURL,
				DurationSec: int(dur),
			})
		}
	case "FAILED":
		task.Status = StatusFailure
		task.FailReason = resp.Message
	default: // ON_QUEUE и другие промежуточные
		task.Status = StatusRunning
	}
	return task, nil
}

// do выполняет HTTP-запрос, маппит статус-коды на доменные ошибки и возвращает тело.
func (c *httpClient) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("TT-API-KEY", c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, ErrInvalidAPIKey
	case http.StatusPaymentRequired:
		return nil, ErrInsufficientCredits
	case http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case http.StatusNotFound:
		return nil, ErrTaskNotFound
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("suno api error: status %d, body: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}
