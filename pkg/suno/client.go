package suno

import (
	"context"
	"errors"
)

// Client - абстрактный контракт клиента для генерации музыки через
// Suno-совместимый API. За этим интерфейсом может стоять обёртка над
// сторонним реселлером (GoAPI, Apiframe, Sunoapi.org и т.п.), собственный
// anti-detect клиент или мок для тестов - пакет suno описывает только форму
// взаимодействия, не способ его реализации.
type Client interface {
	// Generate запускает генерацию и возвращает идентификатор задачи у провайдера.
	Generate(ctx context.Context, req GenerateRequest) (Job, error)

	// GetJob возвращает текущее состояние ранее запущенной задачи.
	GetJob(ctx context.Context, jobID string) (Job, error)
}

// GenerateRequest - параметры запроса генерации в максимально общем виде,
// покрывающем подавляющее большинство Suno-совместимых API.
type GenerateRequest struct {
	Prompt       string
	Style        string
	Title        string
	Instrumental bool
	TrackCount   int
	CallbackURL  string // опционально: webhook, если провайдер его поддерживает
}

// JobStatus - статус задачи генерации на стороне конкретного провайдера.
type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

// Job - состояние задачи генерации.
type Job struct {
	ID       string
	Status   JobStatus
	Tracks   []Track
	ErrorMsg string
}

// Track - один сгенерированный аудио-трек по версии конкретного провайдера.
type Track struct {
	ID          string
	AudioURL    string
	DurationSec int
}

// ErrJobNotFound возвращается, если задача с указанным ID неизвестна клиенту.
var ErrJobNotFound = errors.New("задача генерации не найдена")
