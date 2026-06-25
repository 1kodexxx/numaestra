package openai

import (
	"context"
	"errors"
	"testing"
)

func TestNoopClient_ReturnsErrDisabled(t *testing.T) {
	c := NewNoopClient()
	ctx := context.Background()

	if _, err := c.GenerateLyrics(ctx, "prompt"); !errors.Is(err, ErrDisabled) {
		t.Errorf("GenerateLyrics err = %v", err)
	}
	if _, err := c.GenerateLyricsVariants(ctx, "prompt", 3); !errors.Is(err, ErrDisabled) {
		t.Errorf("GenerateLyricsVariants err = %v", err)
	}
}
