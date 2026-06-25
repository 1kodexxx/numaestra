package openai

import (
	"context"
	"errors"
)

// ErrDisabled — LLM-генерация текстов отключена; Suno пишет слова в Inspiration Mode.
var ErrDisabled = errors.New("openai: LLM-генерация текстов отключена")

// NoopClient — заглушка вместо ChatGPT/OpenRouter, пока тексты пишет Suno.
type NoopClient struct{}

func NewNoopClient() APIClient { return NoopClient{} }

func (NoopClient) GenerateLyrics(_ context.Context, _ string) (string, error) {
	return "", ErrDisabled
}

func (NoopClient) GenerateLyricsVariants(_ context.Context, _ string, _ int) ([]string, error) {
	return nil, ErrDisabled
}
