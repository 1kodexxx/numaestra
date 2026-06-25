package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// jsonMarshal — псевдоним для удобства, чтобы не импортировать encoding/json в каждом методе.
var jsonMarshal = json.Marshal

var (
	ErrCategoryNotFound      = errors.New("категория не найдена")
	ErrCategoryAlreadyExists = errors.New("категория с таким id уже существует")
	ErrQuestionNotFound      = errors.New("вопрос не найден")
)

// CategoryRepository определяет контракт для работы с хранилищем категорий.
// Категории и их вопросы редактируются через Admin API (см. AdminUseCase),
// а не через SQL-миграции — это позволяет добавлять новые сценарии квиза
// (включая "свободный" сценарий без жёсткой категории) без деплоя.
type CategoryRepository interface {
	// GetByID возвращает категорию и связанные с ней вопросы по её ID
	GetByID(ctx context.Context, id string) (*Category, error)

	// GetAll возвращает все категории (базовые данные без вопросов, для главной страницы)
	GetAll(ctx context.Context) ([]*Category, error)

	// Create сохраняет новую категорию. Возвращает ErrCategoryAlreadyExists,
	// если категория с таким ID уже существует.
	Create(ctx context.Context, category *Category) error

	// Update перезаписывает изменяемые поля существующей категории.
	// Возвращает ErrCategoryNotFound, если категории с таким ID нет.
	Update(ctx context.Context, category *Category) error

	// Delete удаляет категорию вместе со всеми её вопросами и вариантами
	// ответов (ON DELETE CASCADE). Возвращает ErrCategoryNotFound, если
	// категории с таким ID нет.
	Delete(ctx context.Context, id string) error

	// AddQuestion добавляет новый вопрос к категории и возвращает вопрос с
	// присвоенным ID. Возвращает ErrCategoryNotFound, если категория не существует.
	AddQuestion(ctx context.Context, categoryID string, q Question) (Question, error)

	// UpdateQuestion перезаписывает вопрос (включая полную замену вариантов
	// ответов). Возвращает ErrQuestionNotFound, если вопрос не относится к
	// указанной категории или не существует.
	UpdateQuestion(ctx context.Context, categoryID string, q Question) error

	// DeleteQuestion удаляет вопрос вместе с его вариантами ответов.
	DeleteQuestion(ctx context.Context, categoryID string, questionID int) error
}

// validUITypes — допустимые значения Question.UIType, понимаемые фронтендом.
var validUITypes = map[string]bool{
	"text": true, "textarea": true, "select": true, "tags": true, "radio": true,
}

// NewCategory создаёт новую категорию каталога после валидации полей.
// id используется как человекочитаемый слаг (например, "wedding", "general") —
// он задаётся администратором, а не генерируется автоматически.
func NewCategory(id, title, description, coverImageURL string, seoTags []string, basePromptTemplate string) (*Category, error) {
	if id == "" {
		return nil, errors.New("id категории обязателен")
	}
	if title == "" {
		return nil, errors.New("название категории обязательно")
	}
	if basePromptTemplate == "" {
		return nil, errors.New("шаблон промпта (base_prompt_template) обязателен")
	}
	return &Category{
		id:                 id,
		title:              title,
		description:        description,
		coverImageURL:      coverImageURL,
		seoTags:            seoTags,
		basePromptTemplate: basePromptTemplate,
	}, nil
}

// UpdateDetails обновляет изменяемые поля категории после валидации.
// ID категории неизменен — для смены идентификатора нужно создать новую категорию.
func (c *Category) UpdateDetails(title, description, coverImageURL string, seoTags []string, basePromptTemplate string) error {
	if title == "" {
		return errors.New("название категории обязательно")
	}
	if basePromptTemplate == "" {
		return errors.New("шаблон промпта (base_prompt_template) обязателен")
	}
	c.title = title
	c.description = description
	c.coverImageURL = coverImageURL
	c.seoTags = seoTags
	c.basePromptTemplate = basePromptTemplate
	return nil
}

// NewQuestion создаёт вопрос квиза после валидации. ID присваивается хранилищем
// при сохранении (AddQuestion) и здесь всегда равен 0.
func NewQuestion(stepNumber int, questionText, uiType, mappingKey string, isRequired bool, optionSource string, config QuestionConfig, options []Option) (Question, error) {
	if questionText == "" {
		return Question{}, errors.New("текст вопроса обязателен")
	}
	if !validUITypes[uiType] {
		return Question{}, fmt.Errorf("недопустимый ui_type %q (допустимо: text, textarea, select, tags, radio)", uiType)
	}
	if mappingKey == "" {
		return Question{}, errors.New("mapping_key обязателен")
	}
	if optionSource == "" {
		optionSource = OptionSourceInline
	}
	if err := validateOptionSource(optionSource); err != nil {
		return Question{}, err
	}
	needsInlineOptions := uiType == "select" || uiType == "tags" || uiType == "radio"
	if needsInlineOptions && optionSource == OptionSourceInline && len(options) == 0 {
		return Question{}, fmt.Errorf("для ui_type %q нужен хотя бы один вариант ответа (options)", uiType)
	}
	return Question{
		StepNumber:   stepNumber,
		QuestionText: questionText,
		UIType:       uiType,
		MappingKey:   mappingKey,
		IsRequired:   isRequired,
		OptionSource: optionSource,
		Config:       config,
		Options:      options,
	}, nil
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
func (c *Category) ID() string                 { return c.id }
func (c *Category) Title() string              { return c.title }
func (c *Category) Description() string        { return c.description }
func (c *Category) CoverImageURL() string      { return c.coverImageURL }
func (c *Category) SeoTags() []string          { return c.seoTags }
func (c *Category) BasePromptTemplate() string { return c.basePromptTemplate }
func (c *Category) Questions() []Question      { return c.questions }

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
	ID           int            `json:"id"`
	StepNumber   int            `json:"step_number"`
	QuestionText string         `json:"question_text"`
	UIType       string         `json:"ui_type"`
	MappingKey   string         `json:"mapping_key"`
	IsRequired   bool           `json:"is_required"`
	OptionSource string         `json:"option_source,omitempty"`
	Config       QuestionConfig `json:"config,omitempty"`
	Options      []Option       `json:"options,omitempty"`
}

type Option struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
