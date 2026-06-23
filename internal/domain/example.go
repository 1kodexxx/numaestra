package domain

import (
	"context"
	"errors"
)

var (
	ErrExampleNotFound      = errors.New("пример не найден")
	ErrExampleAlreadyExists = errors.New("пример с таким id уже существует")
)

// ExampleRepository — порт хранилища примеров готовых работ («Послушать примеры»).
// Примеры редактируются через Admin API, как и категории.
type ExampleRepository interface {
	// GetAll возвращает все примеры (включая скрытые) для админки, в порядке sort_order, id.
	GetAll(ctx context.Context) ([]*Example, error)
	// GetActive возвращает только видимые примеры (is_active) для публичной главной.
	GetActive(ctx context.Context) ([]*Example, error)
	// GetByID возвращает пример по id или ErrExampleNotFound.
	GetByID(ctx context.Context, id string) (*Example, error)
	// Create сохраняет новый пример; ErrExampleAlreadyExists при дубле id.
	Create(ctx context.Context, e *Example) error
	// Update перезаписывает изменяемые поля; ErrExampleNotFound, если примера нет.
	Update(ctx context.Context, e *Example) error
	// Delete удаляет пример; ErrExampleNotFound, если примера нет.
	Delete(ctx context.Context, id string) error
}

// Example — агрегат примера готовой работы. Поля приватные, доступ через геттеры.
type Example struct {
	id          string
	title       string
	category    string
	description string
	mood        string
	audioURL    string
	coverURL    string
	sortOrder   int
	isActive    bool
}

// NewExample создаёт пример после валидации. id — человекочитаемый слаг.
func NewExample(id, title, category, description, mood, audioURL, coverURL string, sortOrder int, isActive bool) (*Example, error) {
	if id == "" {
		return nil, errors.New("id примера обязателен")
	}
	if title == "" {
		return nil, errors.New("название примера обязательно")
	}
	return &Example{
		id:          id,
		title:       title,
		category:    category,
		description: description,
		mood:        mood,
		audioURL:    audioURL,
		coverURL:    coverURL,
		sortOrder:   sortOrder,
		isActive:    isActive,
	}, nil
}

// UpdateDetails обновляет изменяемые поля примера (id неизменен).
func (e *Example) UpdateDetails(title, category, description, mood, audioURL, coverURL string, sortOrder int, isActive bool) error {
	if title == "" {
		return errors.New("название примера обязательно")
	}
	e.title = title
	e.category = category
	e.description = description
	e.mood = mood
	e.audioURL = audioURL
	e.coverURL = coverURL
	e.sortOrder = sortOrder
	e.isActive = isActive
	return nil
}

// ExampleSnapshot — сырые данные для восстановления/сохранения. Только для репозитория.
type ExampleSnapshot struct {
	ID          string
	Title       string
	Category    string
	Description string
	Mood        string
	AudioURL    string
	CoverURL    string
	SortOrder   int
	IsActive    bool
}

// RestoreExample восстанавливает агрегат из снапшота.
func RestoreExample(s ExampleSnapshot) *Example {
	return &Example{
		id:          s.ID,
		title:       s.Title,
		category:    s.Category,
		description: s.Description,
		mood:        s.Mood,
		audioURL:    s.AudioURL,
		coverURL:    s.CoverURL,
		sortOrder:   s.SortOrder,
		isActive:    s.IsActive,
	}
}

// Snapshot возвращает снапшот для слоя хранилища.
func (e *Example) Snapshot() ExampleSnapshot {
	return ExampleSnapshot{
		ID:          e.id,
		Title:       e.title,
		Category:    e.category,
		Description: e.description,
		Mood:        e.mood,
		AudioURL:    e.audioURL,
		CoverURL:    e.coverURL,
		SortOrder:   e.sortOrder,
		IsActive:    e.isActive,
	}
}

// --- Геттеры ---
func (e *Example) ID() string          { return e.id }
func (e *Example) Title() string       { return e.title }
func (e *Example) Category() string    { return e.category }
func (e *Example) Description() string { return e.description }
func (e *Example) Mood() string        { return e.mood }
func (e *Example) AudioURL() string    { return e.audioURL }
func (e *Example) CoverURL() string    { return e.coverURL }
func (e *Example) SortOrder() int      { return e.sortOrder }
func (e *Example) IsActive() bool      { return e.isActive }

// MarshalJSON — плоская публичная форма (snake_case), совпадает с фронтовым ExampleSong.
func (e *Example) MarshalJSON() ([]byte, error) {
	type jsonExample struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Category    string `json:"category"`
		Description string `json:"description"`
		Mood        string `json:"mood"`
		AudioURL    string `json:"audio_url"`
		CoverURL    string `json:"cover_url"`
		SortOrder   int    `json:"sort_order"`
		IsActive    bool   `json:"is_active"`
	}
	return jsonMarshal(jsonExample{
		ID:          e.id,
		Title:       e.title,
		Category:    e.category,
		Description: e.description,
		Mood:        e.mood,
		AudioURL:    e.audioURL,
		CoverURL:    e.coverURL,
		SortOrder:   e.sortOrder,
		IsActive:    e.isActive,
	})
}
