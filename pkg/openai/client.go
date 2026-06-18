package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type APIClient interface {
	GenerateLyrics(ctx context.Context, facts string) (string, error)
}

type httpClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// Теперь мы передаем baseURL, чтобы легко переключиться на OpenRouter
func NewClient(baseURL, apiKey string) APIClient {
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1" // По умолчанию OpenRouter
	}
	return &httpClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 45 * time.Second},
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

func (c *httpClient) GenerateLyrics(ctx context.Context, facts string) (string, error) {
	reqData := chatRequest{
		// Используем дешевую и очень умную модель (можно поменять на anthropic/claude-3-haiku)
		Model: "openai/gpt-4o-mini",
		Messages: []message{
			{
				Role:    "system",
				Content: "Ты гениальный поэт-песенник. Преврати факты пользователя в хит на русском языке. Обязательно используй мета-теги структуры: [Verse 1], [Chorus], [Guitar Solo]. Пиши с юмором, ритмично и без лишних вступлений.",
			},
			{
				Role:    "user",
				Content: facts,
			},
		},
	}

	body, err := json.Marshal(reqData)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	// OpenRouter просит эти заголовки для статистики и ранжирования
	req.Header.Set("HTTP-Referer", "https://numaestra.com")
	req.Header.Set("X-Title", "Numaestra App")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ошибка LLM API: статус %d, ответ: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LLM API вернул пустой ответ")
	}

	return chatResp.Choices[0].Message.Content, nil
}
