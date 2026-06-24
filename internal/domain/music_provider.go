package domain

import (
	"context"
	"errors"
)

// ErrProviderSessionExpired возвращается адаптером провайдера, когда API-ключ или
// сессия аккаунта стали недействительными (HTTP 401). Вызывающий use-case должен
// немедленно вывести аккаунт из ротации и переставить заказ.
var ErrProviderSessionExpired = errors.New("сессия провайдера устарела или недействительна")

// DefaultTrackCount — число версий песни, которое получает клиент за один заказ.
// Продукт фиксированный, без тарифов: всегда 4 версии за один платёж.
const DefaultTrackCount = 4

// DefaultLyricVariantCount — сколько разных текстов песни генерируем на заказ.
// Каждый текст даёт clipsPerTask (2) музыкальные версии → 2×2 = 4 трека.
const DefaultLyricVariantCount = 2

// MusicProvider - порт подсистемы генерации музыки. Домен не знает, реализован
// ли он через прямую интеграцию с Suno, через стороннего API-реселлера или
// через комбинацию нескольких источников - это решение инфраструктурного слоя.
type MusicProvider interface {
	// SubmitGeneration отправляет запрос на генерацию и возвращает
	// идентификатор задачи на стороне провайдера, по которому далее
	// опрашивается результат.
	SubmitGeneration(ctx context.Context, req MusicGenerationRequest) (providerJobID string, err error)

	// FetchResult возвращает текущий статус задачи и треки, если генерация завершена.
	FetchResult(ctx context.Context, providerJobID string) (MusicGenerationResult, error)
}

// MusicGenerationRequest - данные, достаточные для запроса генерации, независимо
// от того, какой конкретный провайдер стоит за MusicProvider.
type MusicGenerationRequest struct {
	Brief        string // один промпт (legacy / fallback)
	Briefs       []string // разные тексты: по одной Suno-задаче на элемент → 2 клипа
	Style        string // жанр/настроение, если выделено отдельно от brief
	Instrumental bool
	TrackCount   int // сколько версий запросить (см. DefaultTrackCount)
}

// MusicGenerationStatus - статус задачи генерации, независимый от провайдера.
type MusicGenerationStatus string

const (
	MusicGenerationStatusPending   MusicGenerationStatus = "pending"
	MusicGenerationStatusRunning   MusicGenerationStatus = "running"
	MusicGenerationStatusCompleted MusicGenerationStatus = "completed"
	MusicGenerationStatusFailed    MusicGenerationStatus = "failed"
)

// MusicGenerationResult - провайдеро-независимый результат опроса задачи.
type MusicGenerationResult struct {
	Status MusicGenerationStatus
	Tracks []ProviderTrack
	Error  string // причина неудачи, если Status == MusicGenerationStatusFailed
}

// ProviderTrack - один трек от провайдера, до скачивания и перезаливки в S3
// Не путать с domain.Track - тот появляется только после Order.Complete().
type ProviderTrack struct {
	SourceURL   string // временная ссылка провайдера, обычно недолговечная
	DurationSec int
	ExternalID  string // идентификатор трека на стороне провайдера, для трассировки
}
