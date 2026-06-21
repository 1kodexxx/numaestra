package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
)

// fakeCategoryCache — счётчик вызовов InvalidateCache для проверки, что
// AdminUseCase сбрасывает кеш каталога после каждой мутации категории/вопроса.
type fakeCategoryCache struct {
	invalidated int
}

func (c *fakeCategoryCache) InvalidateCache() { c.invalidated++ }

func newAdminUCWithCategories(t *testing.T) (*AdminUseCase, *inMemCategoryRepo, *fakeCategoryCache) {
	t.Helper()
	categories := newInMemCategoryRepo()
	cache := &fakeCategoryCache{}
	uc := NewAdminUseCase(newInMemOrderRepo(), newInMemAccountRepo(), categories, &mockRefunder{}, cache, nil, testLogger())
	return uc, categories, cache
}

func TestAdminUseCase_ListCategories_ReturnsAll(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)
	_, _ = uc.CreateCategory(context.Background(), "wedding", "Свадьба", "", "", nil, "шаблон")

	categories, err := uc.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories упал: %v", err)
	}
	if len(categories) != 1 || categories[0].ID() != "wedding" {
		t.Errorf("ожидали 1 категорию wedding, получили %+v", categories)
	}
}

func TestAdminUseCase_GetCategory_NotFound(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)
	_, err := uc.GetCategory(context.Background(), "несуществующая")
	if !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("ожидали ErrCategoryNotFound, получили %v", err)
	}
}

func TestAdminUseCase_CreateCategory_Success(t *testing.T) {
	uc, categories, cache := newAdminUCWithCategories(t)

	cat, err := uc.CreateCategory(context.Background(), "general", "Свободная тема", "Опишите песню своими словами", "/images/general.svg", []string{"свободная", "своя тема"}, "Create a song about: [BRIEF]")
	if err != nil {
		t.Fatalf("CreateCategory упал: %v", err)
	}
	if cat.ID() != "general" {
		t.Errorf("ожидали id=general, получили %q", cat.ID())
	}
	if _, ok := categories.categories["general"]; !ok {
		t.Error("категория должна быть сохранена в репозитории")
	}
	if cache.invalidated != 1 {
		t.Errorf("ожидали 1 вызов InvalidateCache, получили %d", cache.invalidated)
	}
}

func TestAdminUseCase_CreateCategory_DuplicateID(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)

	_, err := uc.CreateCategory(context.Background(), "wedding", "Свадьба", "", "", nil, "Create a [GENRE] song")
	if err != nil {
		t.Fatalf("первое создание не должно падать: %v", err)
	}
	_, err = uc.CreateCategory(context.Background(), "wedding", "Свадьба 2", "", "", nil, "Create a song")
	if !errors.Is(err, domain.ErrCategoryAlreadyExists) {
		t.Fatalf("ожидали ErrCategoryAlreadyExists, получили %v", err)
	}
}

func TestAdminUseCase_CreateCategory_ValidationError(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)

	if _, err := uc.CreateCategory(context.Background(), "", "Заголовок", "", "", nil, "шаблон"); err == nil {
		t.Error("ожидали ошибку валидации при пустом id")
	}
	if _, err := uc.CreateCategory(context.Background(), "id", "", "", "", nil, "шаблон"); err == nil {
		t.Error("ожидали ошибку валидации при пустом title")
	}
	if _, err := uc.CreateCategory(context.Background(), "id", "Заголовок", "", "", nil, ""); err == nil {
		t.Error("ожидали ошибку валидации при пустом base_prompt_template")
	}
}

func TestAdminUseCase_UpdateCategory_Success(t *testing.T) {
	uc, _, cache := newAdminUCWithCategories(t)
	_, err := uc.CreateCategory(context.Background(), "wedding", "Свадьба", "старое описание", "", nil, "шаблон")
	if err != nil {
		t.Fatalf("создание: %v", err)
	}

	updated, err := uc.UpdateCategory(context.Background(), "wedding", "Свадьба (обновлено)", "новое описание", "/img.svg", []string{"тег"}, "новый шаблон")
	if err != nil {
		t.Fatalf("UpdateCategory упал: %v", err)
	}
	if updated.Title() != "Свадьба (обновлено)" || updated.Description() != "новое описание" {
		t.Errorf("поля не обновились: title=%q description=%q", updated.Title(), updated.Description())
	}
	if cache.invalidated != 2 { // 1 при создании + 1 при обновлении
		t.Errorf("ожидали 2 вызова InvalidateCache, получили %d", cache.invalidated)
	}
}

func TestAdminUseCase_UpdateCategory_NotFound(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)
	_, err := uc.UpdateCategory(context.Background(), "несуществующая", "Title", "", "", nil, "шаблон")
	if !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("ожидали ErrCategoryNotFound, получили %v", err)
	}
}

func TestAdminUseCase_DeleteCategory_Success(t *testing.T) {
	uc, categories, cache := newAdminUCWithCategories(t)
	if _, err := uc.CreateCategory(context.Background(), "boss", "Босс", "", "", nil, "шаблон"); err != nil {
		t.Fatalf("создание: %v", err)
	}

	if err := uc.DeleteCategory(context.Background(), "boss"); err != nil {
		t.Fatalf("DeleteCategory упал: %v", err)
	}
	if _, ok := categories.categories["boss"]; ok {
		t.Error("категория должна быть удалена из репозитория")
	}
	if cache.invalidated != 2 {
		t.Errorf("ожидали 2 вызова InvalidateCache, получили %d", cache.invalidated)
	}
}

func TestAdminUseCase_DeleteCategory_NotFound(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)
	err := uc.DeleteCategory(context.Background(), "несуществующая")
	if !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("ожидали ErrCategoryNotFound, получили %v", err)
	}
}

// --- Вопросы ---

func TestAdminUseCase_AddQuestion_Success(t *testing.T) {
	uc, _, cache := newAdminUCWithCategories(t)
	if _, err := uc.CreateCategory(context.Background(), "wedding", "Свадьба", "", "", nil, "шаблон"); err != nil {
		t.Fatalf("создание категории: %v", err)
	}

	q, err := uc.AddQuestion(context.Background(), "wedding", 1, "Как зовут жениха?", "text", "GROOM", true, nil)
	if err != nil {
		t.Fatalf("AddQuestion упал: %v", err)
	}
	if q.ID == 0 {
		t.Error("вопросу должен быть присвоен ID")
	}
	if cache.invalidated != 2 { // создание категории + добавление вопроса
		t.Errorf("ожидали 2 вызова InvalidateCache, получили %d", cache.invalidated)
	}
}

func TestAdminUseCase_AddQuestion_CategoryNotFound(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)
	_, err := uc.AddQuestion(context.Background(), "несуществующая", 1, "Текст?", "text", "KEY", true, nil)
	if !errors.Is(err, domain.ErrCategoryNotFound) {
		t.Fatalf("ожидали ErrCategoryNotFound, получили %v", err)
	}
}

func TestAdminUseCase_AddQuestion_ValidationError(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)
	if _, err := uc.CreateCategory(context.Background(), "wedding", "Свадьба", "", "", nil, "шаблон"); err != nil {
		t.Fatalf("создание категории: %v", err)
	}

	// ui_type "tags" требует options.
	if _, err := uc.AddQuestion(context.Background(), "wedding", 1, "Жанр?", "tags", "GENRE", true, nil); err == nil {
		t.Error("ожидали ошибку валидации: tags без options")
	}
	// недопустимый ui_type.
	if _, err := uc.AddQuestion(context.Background(), "wedding", 1, "Текст?", "checkbox", "KEY", true, nil); err == nil {
		t.Error("ожидали ошибку валидации: недопустимый ui_type")
	}
}

func TestAdminUseCase_UpdateQuestion_Success(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)
	if _, err := uc.CreateCategory(context.Background(), "wedding", "Свадьба", "", "", nil, "шаблон"); err != nil {
		t.Fatalf("создание категории: %v", err)
	}
	q, err := uc.AddQuestion(context.Background(), "wedding", 1, "Как зовут жениха?", "text", "GROOM", true, nil)
	if err != nil {
		t.Fatalf("добавление вопроса: %v", err)
	}

	err = uc.UpdateQuestion(context.Background(), "wedding", q.ID, 2, "Как зовут жениха? (обновлено)", "text", "GROOM", false, nil)
	if err != nil {
		t.Fatalf("UpdateQuestion упал: %v", err)
	}

	cat, err := uc.categoryRepo.GetByID(context.Background(), "wedding")
	if err != nil {
		t.Fatalf("получение категории: %v", err)
	}
	if len(cat.Questions()) != 1 || cat.Questions()[0].QuestionText != "Как зовут жениха? (обновлено)" {
		t.Errorf("вопрос не обновился: %+v", cat.Questions())
	}
}

func TestAdminUseCase_UpdateQuestion_NotFound(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)
	if _, err := uc.CreateCategory(context.Background(), "wedding", "Свадьба", "", "", nil, "шаблон"); err != nil {
		t.Fatalf("создание категории: %v", err)
	}
	err := uc.UpdateQuestion(context.Background(), "wedding", 999, 1, "Текст?", "text", "KEY", true, nil)
	if !errors.Is(err, domain.ErrQuestionNotFound) {
		t.Fatalf("ожидали ErrQuestionNotFound, получили %v", err)
	}
}

func TestAdminUseCase_DeleteQuestion_Success(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)
	if _, err := uc.CreateCategory(context.Background(), "wedding", "Свадьба", "", "", nil, "шаблон"); err != nil {
		t.Fatalf("создание категории: %v", err)
	}
	q, err := uc.AddQuestion(context.Background(), "wedding", 1, "Как зовут жениха?", "text", "GROOM", true, nil)
	if err != nil {
		t.Fatalf("добавление вопроса: %v", err)
	}

	if err := uc.DeleteQuestion(context.Background(), "wedding", q.ID); err != nil {
		t.Fatalf("DeleteQuestion упал: %v", err)
	}

	cat, err := uc.categoryRepo.GetByID(context.Background(), "wedding")
	if err != nil {
		t.Fatalf("получение категории: %v", err)
	}
	if len(cat.Questions()) != 0 {
		t.Errorf("вопрос должен быть удалён, осталось: %+v", cat.Questions())
	}
}

func TestAdminUseCase_DeleteQuestion_NotFound(t *testing.T) {
	uc, _, _ := newAdminUCWithCategories(t)
	if _, err := uc.CreateCategory(context.Background(), "wedding", "Свадьба", "", "", nil, "шаблон"); err != nil {
		t.Fatalf("создание категории: %v", err)
	}
	err := uc.DeleteQuestion(context.Background(), "wedding", 999)
	if !errors.Is(err, domain.ErrQuestionNotFound) {
		t.Fatalf("ожидали ErrQuestionNotFound, получили %v", err)
	}
}
