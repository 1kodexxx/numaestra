package suno

import (
	"context"
	"fmt"

	"github.com/numaestra/numaestra/internal/domain"
	pkgsuno "github.com/numaestra/numaestra/pkg/suno"
)

// ProviderAdapter реализует domain.MusicProvider, транслируя доменные понятия
// в провайдеро-независимый контракт pkg/suno.Client. Какой именно провайдер
// стоит за этим адаптером (реселлер, свой клиент, мок) определяется тем,
// какая реализация pkg/suno.Client передана в конструктор - это единственное
// место, которое нужно будет поменять при смене провайдера.
type ProviderAdapter struct {
	client pkgsuno.Client
}

func NewProviderAdapter(client pkgsuno.Client) *ProviderAdapter {
	return &ProviderAdapter{client: client}
}

var _ domain.MusicProvider = (*ProviderAdapter)(nil)

func (a *ProviderAdapter) SubmitGeneration(ctx context.Context, req domain.MusicGenerationRequest) (string, error) {
	job, err := a.client.Generate(ctx, pkgsuno.GenerateRequest{
		Prompt:       req.Brief,
		Style:        req.Style,
		Instrumental: req.Instrumental,
		TrackCount:   req.TrackCount,
	})
	if err != nil {
		return "", fmt.Errorf("запрос генерации у провайдера: %w", err)
	}
	return job.ID, nil
}

func (a *ProviderAdapter) FetchResult(ctx context.Context, providerJobID string) (domain.MusicGenerationResult, error) {
	job, err := a.client.GetJob(ctx, providerJobID)
	if err != nil {
		return domain.MusicGenerationResult{}, fmt.Errorf("получение статуса задачи у провайдера: %w", err)
	}

	tracks := make([]domain.ProviderTrack, len(job.Tracks))
	for i, t := range job.Tracks {
		tracks[i] = domain.ProviderTrack{
			SourceURL:   t.AudioURL,
			DurationSec: t.DurationSec,
			ExternalID:  t.ID,
		}
	}

	return domain.MusicGenerationResult{
		Status: mapStatus(job.Status),
		Tracks: tracks,
		Error:  job.ErrorMsg,
	}, nil
}

func mapStatus(s pkgsuno.JobStatus) domain.MusicGenerationStatus {
	switch s {
	case pkgsuno.JobStatusQueued:
		return domain.MusicGenerationStatusPending
	case pkgsuno.JobStatusRunning:
		return domain.MusicGenerationStatusRunning
	case pkgsuno.JobStatusCompleted:
		return domain.MusicGenerationStatusCompleted
	default:
		return domain.MusicGenerationStatusFailed
	}
}
