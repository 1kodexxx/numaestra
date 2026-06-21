package sunorepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/circuitbreaker"
	"github.com/numaestra/numaestra/pkg/suno"
)

type ProviderAdapter struct {
	client  suno.APIClient
	breaker *circuitbreaker.Breaker
}

// NewProviderAdapter создаёт адаптер с circuit breaker:
// 5 последовательных ошибок → размыкание на 30 секунд.
func NewProviderAdapter(client suno.APIClient) *ProviderAdapter {
	return &ProviderAdapter{
		client:  client,
		breaker: circuitbreaker.New("suno", 5, 30*time.Second),
	}
}

var _ domain.MusicProvider = (*ProviderAdapter)(nil)

func (a *ProviderAdapter) SubmitGeneration(ctx context.Context, req domain.MusicGenerationRequest) (string, error) {
	apiReq := suno.GenerateRequest{
		Prompt:           req.Brief,
		MakeInstrumental: req.Instrumental,
		WaitAudio:        false,
	}

	var clips []suno.Clip
	if err := a.breaker.Do(func() error {
		var err error
		clips, err = a.client.Generate(ctx, apiReq)
		return err
	}); err != nil {
		if errors.Is(err, suno.ErrSessionExpired) {
			return "", domain.ErrProviderSessionExpired
		}
		return "", fmt.Errorf("ошибка генерации в Suno: %w", err)
	}

	if len(clips) == 0 {
		return "", fmt.Errorf("провайдер вернул пустой список клипов")
	}

	var ids []string
	for _, clip := range clips {
		ids = append(ids, clip.ID)
	}

	providerJobID := strings.Join(ids, ",")
	return providerJobID, nil
}

func (a *ProviderAdapter) FetchResult(ctx context.Context, providerJobID string) (domain.MusicGenerationResult, error) {
	if providerJobID == "" {
		return domain.MusicGenerationResult{Status: domain.MusicGenerationStatusFailed, Error: "отсутствует provider job id"}, nil
	}

	ids := strings.Split(providerJobID, ",")
	var clips []suno.Clip
	if err := a.breaker.Do(func() error {
		var err error
		clips, err = a.client.GetFeed(ctx, ids)
		return err
	}); err != nil {
		return domain.MusicGenerationResult{}, fmt.Errorf("ошибка опроса статуса Suno: %w", err)
	}

	result := domain.MusicGenerationResult{
		Status: domain.MusicGenerationStatusCompleted,
		Tracks: make([]domain.ProviderTrack, 0, len(clips)),
	}

	allCompleted := true
	hasErrors := false
	var lastError string

	for _, clip := range clips {
		switch clip.Status {
		case suno.StatusError:
			hasErrors = true
			lastError = clip.ErrorMessage
		case suno.StatusComplete:
			result.Tracks = append(result.Tracks, domain.ProviderTrack{
				SourceURL:   clip.AudioURL,
				DurationSec: int(clip.Duration),
				ExternalID:  clip.ID,
			})
		default:
			allCompleted = false
		}
	}

	if hasErrors {
		result.Status = domain.MusicGenerationStatusFailed
		result.Error = lastError
	} else if !allCompleted {
		result.Status = domain.MusicGenerationStatusRunning
	}

	return result, nil
}
