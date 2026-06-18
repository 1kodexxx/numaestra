package sunorepo

import (
	"context"
	"fmt"
	"strings"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/suno"
)

type ProviderAdapter struct {
	client suno.APIClient
}

func NewProviderAdapter(client suno.APIClient) *ProviderAdapter {
	return &ProviderAdapter{
		client: client,
	}
}

var _ domain.MusicProvider = (*ProviderAdapter)(nil)

func (a *ProviderAdapter) SubmitGeneration(ctx context.Context, req domain.MusicGenerationRequest) (string, error) {
	apiReq := suno.GenerateRequest{
		Prompt:           req.Brief,
		MakeInstrumental: req.Instrumental,
		WaitAudio:        false,
	}

	clips, err := a.client.Generate(ctx, apiReq)
	if err != nil {
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
	clips, err := a.client.GetFeed(ctx, ids)
	if err != nil {
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
