package domain

import "context"

// CategoryRepository определяет контракт для работы с хранилищем категорий
type CategoryRepository interface {
	// GetByID возвращает категорию и связанные с ней вопросы по её ID
	GetByID(ctx context.Context, id string) (*Category, error)

	// Скорее всего, вам сразу понадобится метод для получения всех категорий
	// для отображения карточек на главной странице фронтенда
	GetAll(ctx context.Context) ([]*Category, error)
}

type Category struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	CoverImageURL      string     `json:"cover_image_url"`
	SeoTags            []string   `json:"seo_tags"`
	BasePromptTemplate string     `json:"-"`
	Questions          []Question `json:"questions,omitempty"`
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
