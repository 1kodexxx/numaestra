// pkg/suno/client.go
//
// Клиент Sunor.cc Suno API (https://docs.sunor.cc). Модель асинхронная: создаём
// задачу (POST /api/v1/task) → получаем task_id → опрашиваем её (GET
// /api/v1/task/{id}), пока статус не станет терминальным. Авторизация — заголовок
// x-api-key (один ключ на аккаунт, без cookie-сессий).
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
	"strings"
	"time"
)

// Терминальные и промежуточные статусы задачи Sunor (поле data.status).
const (
	StatusPending = "pending" // принята, ждёт обработки
	StatusRunning = "running" // обрабатывается провайдером
	StatusSuccess = "success" // готово, output.result заполнен
	StatusFailure = "failure" // ошибка; кредиты возвращены
	StatusTimeout = "timeout" // таймаут; кредиты возвращены
)

// Доменно-значимые ошибки API. Вызывающий код различает их через errors.Is,
// чтобы по-разному реагировать (вывести аккаунт из ротации, не списывать ретрай и т.п.).
var (
	// ErrInvalidAPIKey — 401: ключ отсутствует или недействителен. Аккаунт нужно
	// немедленно вывести из ротации до обновления ключа.
	ErrInvalidAPIKey = errors.New("suno: неверный или отсутствующий API-ключ (401)")
	// ErrInsufficientCredits — 402: на балансе ключа недостаточно кредитов.
	ErrInsufficientCredits = errors.New("suno: недостаточно кредитов на балансе (402)")
	// ErrRateLimited — 429: превышен лимит запросов (по умолчанию 120 req/мин на ключ).
	ErrRateLimited = errors.New("suno: превышен лимит запросов (429)")
	// ErrTaskNotFound — 404: задача не найдена (неверный id или чужой аккаунт).
	ErrTaskNotFound = errors.New("suno: задача не найдена (404)")
)

// APIClient — порт клиента Sunor для адаптера провайдера.
type APIClient interface {
	// CreateMusicTask создаёт задачу генерации музыки и возвращает её task_id.
	CreateMusicTask(ctx context.Context, in MusicInput) (taskID string, err error)
	// GetTask возвращает текущее состояние задачи: статус и (по готовности) клипы.
	GetTask(ctx context.Context, taskID string) (Task, error)
}

// MusicInput — параметры генерации. Используется Inspiration Mode Sunor:
// gpt_description_prompt свободным текстом, провайдер сам пишет текст и музыку.
type MusicInput struct {
	Description  string // gpt_description_prompt — описание/бриф будущей песни
	Tags         string // input.tags — стиль/жанр через запятую (опционально)
	Instrumental bool   // make_instrumental — песня без вокала
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
	client  *http.Client
}

// NewClient создаёт клиент Sunor. baseURL — корень API (например, https://sunor.cc),
// к нему добавляется /api/v1/...; apiKey уходит в заголовок x-api-key.
func NewClient(baseURL, apiKey string) APIClient {
	return &httpClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- DTO внешнего API (snake_case согласно docs.sunor.cc) ---

type createTaskRequest struct {
	Model    string         `json:"model"`
	TaskType string         `json:"task_type"`
	Input    musicInputBody `json:"input"`
}

type musicInputBody struct {
	GptDescriptionPrompt string `json:"gpt_description_prompt"`
	Tags                 string `json:"tags,omitempty"`
	MakeInstrumental     bool   `json:"make_instrumental"`
}

type createTaskResponse struct {
	Code int `json:"code"`
	Data struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	} `json:"data"`
}

type getTaskResponse struct {
	Code int `json:"code"`
	Data struct {
		TaskID string          `json:"task_id"`
		Status string          `json:"status"`
		Error  json.RawMessage `json:"error"`
		Output struct {
			FailReason *string      `json:"fail_reason"`
			Result     []clipResult `json:"result"`
		} `json:"output"`
	} `json:"data"`
}

type clipResult struct {
	ID       string `json:"id"`
	AudioURL string `json:"audio_url"`
	Metadata struct {
		Duration float64 `json:"duration"`
	} `json:"metadata"`
}

// CreateMusicTask отправляет POST /api/v1/task в Inspiration Mode.
func (c *httpClient) CreateMusicTask(ctx context.Context, in MusicInput) (string, error) {
	body, err := json.Marshal(createTaskRequest{
		Model:    "suno",
		TaskType: "music",
		Input: musicInputBody{
			GptDescriptionPrompt: in.Description,
			Tags:                 in.Tags,
			MakeInstrumental:     in.Instrumental,
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshal create task: %w", err)
	}

	raw, err := c.do(ctx, http.MethodPost, "/api/v1/task", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	var resp createTaskResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("decode create task: %w", err)
	}
	if resp.Data.TaskID == "" {
		return "", fmt.Errorf("suno: пустой task_id в ответе")
	}
	return resp.Data.TaskID, nil
}

// GetTask опрашивает GET /api/v1/task/{taskID}.
func (c *httpClient) GetTask(ctx context.Context, taskID string) (Task, error) {
	if taskID == "" {
		return Task{}, fmt.Errorf("suno: пустой task id")
	}

	raw, err := c.do(ctx, http.MethodGet, "/api/v1/task/"+url.PathEscape(taskID), nil)
	if err != nil {
		return Task{}, err
	}

	var resp getTaskResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return Task{}, fmt.Errorf("decode get task: %w", err)
	}

	task := Task{
		ID:         resp.Data.TaskID,
		Status:     resp.Data.Status,
		FailReason: failReason(resp.Data.Output.FailReason, resp.Data.Error),
	}
	for _, r := range resp.Data.Output.Result {
		task.Clips = append(task.Clips, Clip{
			ID:          r.ID,
			AudioURL:    r.AudioURL,
			DurationSec: int(r.Metadata.Duration),
		})
	}
	if task.ID == "" {
		task.ID = taskID
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
		req.Header.Set("x-api-key", c.apiKey)
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
	// Создание задачи отвечает 200/202, опрос — 200. Любой иной 2xx считаем успехом.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("suno api error: status %d, body: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

// failReason извлекает человекочитаемую причину сбоя: сначала output.fail_reason,
// затем data.error (строка либо произвольный JSON).
func failReason(failReason *string, rawError json.RawMessage) string {
	if failReason != nil && *failReason != "" {
		return *failReason
	}
	if len(rawError) == 0 || string(rawError) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(rawError, &s); err == nil {
		return s
	}
	return string(rawError)
}
