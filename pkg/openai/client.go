package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const lyricVariantSeparator = "---VARIANT---"

type APIClient interface {
	GenerateLyrics(ctx context.Context, facts string) (string, error)
	// GenerateLyricsVariants возвращает count различных текстов песни по одному брифу.
	GenerateLyricsVariants(ctx context.Context, facts string, count int) ([]string, error)
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

const lyricSystemPrompt = "You write song lyrics optimized for Suno AI music generation. " +
	"Output ONLY lyrics with English section tags: [Verse 1], [Verse 2], [Chorus], [Bridge], [Outro]. " +
	"All sung lines must be in Russian. Keep lines rhythmic and singable (about 6–10 syllables). " +
	"Use every name and fact from the user's brief. Match genre, mood, tempo, and vocals from the brief. " +
	"No titles, explanations, or markdown."

func (c *httpClient) GenerateLyrics(ctx context.Context, facts string) (string, error) {
	reqData := chatRequest{
		// Используем дешевую и очень умную модель (можно поменять на anthropic/claude-3-haiku)
		Model: "openai/gpt-4o-mini",
		Messages: []message{
			{
				Role:    "system",
				Content: lyricSystemPrompt,
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

func (c *httpClient) GenerateLyricsVariants(ctx context.Context, facts string, count int) ([]string, error) {
	if count <= 0 {
		count = 1
	}
	if count == 1 {
		one, err := c.GenerateLyrics(ctx, facts)
		if err != nil {
			return nil, err
		}
		return []string{one}, nil
	}

	reqData := chatRequest{
		Model: "openai/gpt-4o-mini",
		Messages: []message{
			{
				Role: "system",
				Content: fmt.Sprintf(
					"You write song lyrics for Suno AI. Write %d DIFFERENT Russian song variants from the user's brief. "+
						"Each variant: full song with English tags [Verse 1], [Chorus], etc. "+
						"Variants must differ in chorus and rhymes but share the same theme, mood, and style. "+
						"Separate variants with %s on its own line. No introductions.",
					count, lyricVariantSeparator,
				),
			},
			{Role: "user", Content: facts},
		},
	}

	body, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("HTTP-Referer", "https://numaestra.com")
	req.Header.Set("X-Title", "Numaestra App")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ошибка LLM API: статус %d, ответ: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, err
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM API вернул пустой ответ")
	}

	variants := splitLyricVariants(chatResp.Choices[0].Message.Content)
	for len(variants) < count {
		alt, err := c.GenerateLyrics(ctx, facts+"\n\nНапиши АЛЬТЕРНАТИВНЫЙ вариант с другим припевом, другими рифмами и формулировками. Не повторяй предыдущие строки.")
		if err != nil {
			if len(variants) == 0 {
				return nil, err
			}
			break
		}
		if !containsVariant(variants, alt) {
			variants = append(variants, alt)
		}
	}
	if len(variants) > count {
		variants = variants[:count]
	}
	if len(variants) == 0 {
		return nil, fmt.Errorf("LLM не вернул ни одного варианта текста")
	}
	return variants, nil
}

func splitLyricVariants(content string) []string {
	parts := strings.Split(content, lyricVariantSeparator)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func containsVariant(variants []string, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	for _, v := range variants {
		if strings.TrimSpace(v) == candidate {
			return true
		}
	}
	return false
}
