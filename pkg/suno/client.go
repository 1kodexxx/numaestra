// pkg/suno/client.go
package suno

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrSessionExpired возвращается, когда Suno API отвечает 401: сессия протухла
// или была отозвана. Вызывающий код должен немедленно вывести аккаунт из ротации.
var ErrSessionExpired = errors.New("suno: сессия устарела или недействительна (401)")

// Клиентские константы статусов Suno API
const (
	StatusSubmitted = "submitted"
	StatusQueued    = "queued"
	StatusStreaming = "streaming"
	StatusComplete  = "complete"
	StatusError     = "error"
)

type APIClient interface {
	Generate(ctx context.Context, req GenerateRequest) ([]Clip, error)
	GetFeed(ctx context.Context, ids []string) ([]Clip, error)
}

type httpClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewClient(baseURL, apiKey string) APIClient {
	return &httpClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second, // Адекватный таймаут для внешних интеграций
		},
	}
}

// DTO контракты внешнего API

type GenerateRequest struct {
	Prompt           string `json:"prompt"`
	Tags             string `json:"tags,omitempty"`
	Title            string `json:"title,omitempty"`
	MakeInstrumental bool   `json:"make_instrumental"`
	WaitAudio        bool   `json:"wait_audio"` // Обычно false для асинхронной работы
}

type Clip struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	AudioURL     string  `json:"audio_url"`
	VideoURL     string  `json:"video_url,omitempty"`
	Duration     float64 `json:"duration,omitempty"`
	Title        string  `json:"title,omitempty"`
	Tags         string  `json:"tags,omitempty"`
	ErrorMessage string  `json:"error_message,omitempty"`
}

// Generate инициирует процесс генерации музыки. Возвращает массив предварительных данных (обычно 2 клипа).
func (c *httpClient) Generate(ctx context.Context, req GenerateRequest) ([]Clip, error) {
	endpoint := fmt.Sprintf("%s/api/custom_generate", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrSessionExpired
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var clips []Clip
	if err := json.NewDecoder(resp.Body).Decode(&clips); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return clips, nil
}

// GetFeed запрашивает актуальный статус сгенерированных треков по их идентификаторами.
func (c *httpClient) GetFeed(ctx context.Context, ids []string) ([]Clip, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	endpoint := fmt.Sprintf("%s/api/get?ids=%s", c.baseURL, strings.Join(ids, ","))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrSessionExpired
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: status %d, body: %s", resp.StatusCode, string(respBody))
	}

	var clips []Clip
	if err := json.NewDecoder(resp.Body).Decode(&clips); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return clips, nil
}
