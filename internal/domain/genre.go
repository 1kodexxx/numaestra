package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrGenreNotFound      = errors.New("жанр не найден")
	ErrGenreAlreadyExists = errors.New("жанр с таким slug уже существует")
)

const OptionSourceInline = "inline"
const OptionSourceGenres = "genres"

// GenreRepository — справочник музыкальных жанров для квиза и конструктора.
type GenreRepository interface {
	GetAll(ctx context.Context, activeOnly bool) ([]Genre, error)
	GetByID(ctx context.Context, id int) (*Genre, error)
	GetForCategory(ctx context.Context, categoryID string) ([]Genre, error)
	Create(ctx context.Context, g *Genre) error
	Update(ctx context.Context, g *Genre) error
	Delete(ctx context.Context, id int) error
	SetCategoryGenres(ctx context.Context, categoryID string, genreIDs []int) error
	GetCategoryGenreIDs(ctx context.Context, categoryID string) ([]int, error)
}

// Genre — элемент справочника жанров.
type Genre struct {
	ID        int    `json:"id"`
	Slug      string `json:"slug"`
	Label     string `json:"label"`
	SunoValue string `json:"suno_value"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

func NewGenre(slug, label, sunoValue string, sortOrder int) (*Genre, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return nil, errors.New("slug жанра обязателен")
	}
	if label = strings.TrimSpace(label); label == "" {
		return nil, errors.New("название жанра обязательно")
	}
	if sunoValue = strings.TrimSpace(sunoValue); sunoValue == "" {
		return nil, errors.New("suno_value обязателен")
	}
	return &Genre{Slug: slug, Label: label, SunoValue: sunoValue, SortOrder: sortOrder, IsActive: true}, nil
}

func (g *Genre) Update(label, sunoValue string, sortOrder int, isActive bool) error {
	if strings.TrimSpace(label) == "" {
		return errors.New("название жанра обязательно")
	}
	if strings.TrimSpace(sunoValue) == "" {
		return errors.New("suno_value обязателен")
	}
	g.Label = strings.TrimSpace(label)
	g.SunoValue = strings.TrimSpace(sunoValue)
	g.SortOrder = sortOrder
	g.IsActive = isActive
	return nil
}

func (g *Genre) ToOption() Option {
	return Option{Label: g.Label, Value: g.SunoValue}
}

// QuestionConfig — метаданные вопроса (подсказки, лимиты выбора).
type QuestionConfig struct {
	Placeholder string `json:"placeholder,omitempty"`
	Hint        string `json:"hint,omitempty"`
	MinSelect   int    `json:"min_select,omitempty"`
	MaxSelect   int    `json:"max_select,omitempty"`
}

func ParseQuestionConfig(raw map[string]any) QuestionConfig {
	var cfg QuestionConfig
	if raw == nil {
		return cfg
	}
	if v, ok := raw["placeholder"].(string); ok {
		cfg.Placeholder = v
	}
	if v, ok := raw["hint"].(string); ok {
		cfg.Hint = v
	}
	if v, ok := raw["min_select"].(float64); ok {
		cfg.MinSelect = int(v)
	}
	if v, ok := raw["max_select"].(float64); ok {
		cfg.MaxSelect = int(v)
	}
	return cfg
}

func (c QuestionConfig) ToMap() map[string]any {
	m := map[string]any{}
	if c.Placeholder != "" {
		m["placeholder"] = c.Placeholder
	}
	if c.Hint != "" {
		m["hint"] = c.Hint
	}
	if c.MinSelect > 0 {
		m["min_select"] = c.MinSelect
	}
	if c.MaxSelect > 0 {
		m["max_select"] = c.MaxSelect
	}
	if len(m) == 0 {
		return map[string]any{}
	}
	return m
}

func validateOptionSource(src string) error {
	switch src {
	case "", OptionSourceInline, OptionSourceGenres:
		return nil
	default:
		return fmt.Errorf("недопустимый option_source %q", src)
	}
}
