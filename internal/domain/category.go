package domain

import (
	"context"
	"encoding/json"
)

// jsonMarshal — псевдоним для удобства, чтобы не импортировать encoding/json в каждом методе.
var jsonMarshal = json.Marshal

// CategoryRepository определяет контракт для работы с хранилищем категорий
type CategoryRepository interface {
	// GetByID возвращает категорию и связанные с ней вопросы по её ID
	GetByID(ctx context.Context, id string) (*Category, error)

	// GetAll возвращает все категории (базовые данные без вопросов, для главной страницы)
	GetAll(ctx context.Context) ([]*Category, error)
}

// Category — агрегат каталога: категория песни со списком вопросов квиза.
// Поля приватные — доступ только через геттеры, мутации — только через RestoreCategory.
// Это гарантирует, что инфраструктурный слой не присваивает поля напрямую.
type Category struct {
	id                 string
	title              string
	description        string
	coverImageURL      string
	seoTags            []string
	basePromptTemplate string
	questions          []Question
}

// CategorySnapshot — сырые данные для восстановления агрегата из хранилища.
// Используется только репозиторием.
type CategorySnapshot struct {
	ID                 string
	Title              string
	Description        string
	CoverImageURL      string
	SeoTags            []string
	BasePromptTemplate string
	Questions          []Question
}

// RestoreCategory восстанавливает агрегат из снапшота.
func RestoreCategory(s CategorySnapshot) *Category {
	return &Category{
		id:                 s.ID,
		title:              s.Title,
		description:        s.Description,
		coverImageURL:      s.CoverImageURL,
		seoTags:            s.SeoTags,
		basePromptTemplate: s.BasePromptTemplate,
		questions:          s.Questions,
	}
}

// Snapshot возвращает снапшот агрегата для слоя хранилища.
func (c *Category) Snapshot() CategorySnapshot {
	return CategorySnapshot{
		ID:                 c.id,
		Title:              c.title,
		Description:        c.description,
		CoverImageURL:      c.coverImageURL,
		SeoTags:            c.seoTags,
		BasePromptTemplate: c.basePromptTemplate,
		Questions:          c.questions,
	}
}

// --- Геттеры ---
func (c *Category) ID() string                   { return c.id }
func (c *Category) Title() string                { return c.title }
func (c *Category) Description() string          { return c.description }
func (c *Category) CoverImageURL() string        { return c.coverImageURL }
func (c *Category) SeoTags() []string            { return c.seoTags }
func (c *Category) BasePromptTemplate() string   { return c.basePromptTemplate }
func (c *Category) Questions() []Question        { return c.questions }

// MarshalJSON реализует кастомную сериализацию для публичного API:
// возвращает плоскую структуру с JSON-тегами, не раскрывая basePromptTemplate.
func (c *Category) MarshalJSON() ([]byte, error) {
	type jsonCategory struct {
		ID            string     `json:"id"`
		Title         string     `json:"title"`
		Description   string     `json:"description"`
		CoverImageURL string     `json:"cover_image_url"`
		SeoTags       []string   `json:"seo_tags"`
		Questions     []Question `json:"questions,omitempty"`
	}
	return jsonMarshal(jsonCategory{
		ID:            c.id,
		Title:         c.title,
		Description:   c.description,
		CoverImageURL: c.coverImageURL,
		SeoTags:       c.seoTags,
		Questions:     c.questions,
	})
}

type Question struct {
	ID           int      `json:"id"`
	StepNumber   int      `json:"step_number"`
	QuestionText string   `json:"question_text"`
	UIType       string   `json:"ui_type"`
	MappingKey   string   `json:"mapping_key"`
	IsRequired   bool     `json:"is_required"`
	Options      []Option `json:"options,omitempty"`
}

type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
