package sunorepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/suno"
)

// clipsPerTask — сколько клипов отдаёт одна музыкальная задача Sunor. Suno
// генерирует две вариации за вызов, поэтому для продуктовых 4 версий адаптер
// создаёт несколько задач (см. SubmitGeneration).
const clipsPerTask = 2

// ProviderAdapter реализует domain.MusicProvider поверх клиента Sunor.cc.
// providerJobID — это один или несколько task_id, склеенных запятой: домен видит
// единый идентификатор, а адаптер прячет за ним пачку задач Sunor.
type ProviderAdapter struct {
	client suno.APIClient
}

func NewProviderAdapter(client suno.APIClient) *ProviderAdapter {
	return &ProviderAdapter{client: client}
}

var _ domain.MusicProvider = (*ProviderAdapter)(nil)

// SubmitGeneration создаёт столько задач Sunor, сколько нужно для req.TrackCount
// версий (по clipsPerTask на задачу), и возвращает их task_id через запятую.
// Возвращает ошибку только если НИ одну задачу создать не удалось — частичный
// успех допустим (лучше выдать меньше версий, чем провалить весь заказ).
func (a *ProviderAdapter) SubmitGeneration(ctx context.Context, req domain.MusicGenerationRequest) (string, error) {
	jobs := generationJobs(req)
	var taskIDs []string
	var firstErr error

	for _, in := range jobs {
		taskID, err := a.client.CreateMusicTask(ctx, in)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		taskIDs = append(taskIDs, taskID)
	}

	if len(taskIDs) == 0 {
		// Все задачи не создались — пробрасываем причину. 401 (недействительный
		// ключ) маппим в доменную ошибку, чтобы use-case вывел аккаунт из ротации.
		if errors.Is(firstErr, suno.ErrInvalidAPIKey) {
			return "", domain.ErrProviderSessionExpired
		}
		return "", fmt.Errorf("ошибка создания задачи в Suno: %w", firstErr)
	}

	return strings.Join(taskIDs, ","), nil
}

// FetchResult опрашивает все задачи providerJobID и агрегирует их в единый
// результат: completed — когда все задачи терминальны и есть хоть один клип;
// running — пока хоть одна не завершилась; failed — когда все завершились без клипов.
func (a *ProviderAdapter) FetchResult(ctx context.Context, providerJobID string) (domain.MusicGenerationResult, error) {
	if providerJobID == "" {
		return domain.MusicGenerationResult{Status: domain.MusicGenerationStatusFailed, Error: "отсутствует provider job id"}, nil
	}

	taskIDs := strings.Split(providerJobID, ",")
	var tracks []domain.ProviderTrack
	var lastFailReason string
	stillRunning := false
	var taskScore float64
	clipsReady := 0

	for _, taskID := range taskIDs {
		task, err := a.client.GetTask(ctx, taskID)
		if err != nil {
			return domain.MusicGenerationResult{}, fmt.Errorf("ошибка опроса статуса Suno (task %s): %w", taskID, err)
		}

		switch task.Status {
		case suno.StatusSuccess:
			taskScore += 1.0
			for _, clip := range task.Clips {
				clipsReady++
				tracks = append(tracks, domain.ProviderTrack{
					SourceURL:   clip.AudioURL,
					DurationSec: clip.DurationSec,
					ExternalID:  clip.ID,
				})
			}
		case suno.StatusFailure, suno.StatusTimeout:
			if task.FailReason != "" {
				lastFailReason = task.FailReason
			}
		case suno.StatusRunning:
			stillRunning = true
			taskScore += 0.55
		default: // pending
			stillRunning = true
			taskScore += 0.12
		}
	}

	if stillRunning {
		return domain.MusicGenerationResult{
			Status:          domain.MusicGenerationStatusRunning,
			ProgressPercent: runningProgressPercent(taskScore, len(taskIDs)),
			ClipsReady:      clipsReady,
		}, nil
	}

	// Все задачи терминальны. Есть клипы — успех (даже если часть задач упала);
	// клипов нет — отказ.
	if len(tracks) == 0 {
		if lastFailReason == "" {
			lastFailReason = "провайдер не вернул ни одного трека"
		}
		return domain.MusicGenerationResult{Status: domain.MusicGenerationStatusFailed, Error: lastFailReason}, nil
	}

	return domain.MusicGenerationResult{Status: domain.MusicGenerationStatusCompleted, Tracks: tracks}, nil
}

// runningProgressPercent переводит долю завершённых Suno-задач в 30–85% шкалы заказа.
func runningProgressPercent(taskScore float64, taskCount int) int {
	if taskCount <= 0 {
		return 30
	}
	const base, max = 30, 85
	ratio := taskScore / float64(taskCount)
	if ratio > 1 {
		ratio = 1
	}
	return base + int(ratio*float64(max-base))
}

// tasksFor возвращает число задач Sunor, нужное для trackCount версий (минимум 1).
func tasksFor(trackCount int) int {
	if trackCount <= clipsPerTask {
		return 1
	}
	return (trackCount + clipsPerTask - 1) / clipsPerTask
}
