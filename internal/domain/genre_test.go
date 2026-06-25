package domain

import (
	"testing"
)

func TestNewGenre_Valid(t *testing.T) {
	g, err := NewGenre("Pop", " Поп ", "modern pop", 10)
	if err != nil {
		t.Fatalf("ожидали успех: %v", err)
	}
	if g.Slug != "pop" || g.Label != "Поп" || g.SunoValue != "modern pop" || !g.IsActive {
		t.Errorf("неверные поля: %+v", g)
	}
}

func TestNewGenre_Validation(t *testing.T) {
	if _, err := NewGenre("", "Поп", "pop", 0); err == nil {
		t.Error("ожидали ошибку при пустом slug")
	}
	if _, err := NewGenre("pop", "", "pop", 0); err == nil {
		t.Error("ожидали ошибку при пустом label")
	}
	if _, err := NewGenre("pop", "Поп", "", 0); err == nil {
		t.Error("ожидали ошибку при пустом suno_value")
	}
}

func TestGenre_Update(t *testing.T) {
	g, _ := NewGenre("rock", "Рок", "rock", 10)
	if err := g.Update("Рок-н-ролл", "classic rock", 20, false); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if g.Label != "Рок-н-ролл" || g.SunoValue != "classic rock" || g.SortOrder != 20 || g.IsActive {
		t.Errorf("поля не обновились: %+v", g)
	}
}

func TestGenre_ToOption(t *testing.T) {
	g, _ := NewGenre("jazz", "Джаз", "smooth jazz", 0)
	opt := g.ToOption()
	if opt.Label != "Джаз" || opt.Value != "smooth jazz" {
		t.Errorf("ToOption: %+v", opt)
	}
}

func TestParseQuestionConfig(t *testing.T) {
	cfg := ParseQuestionConfig(map[string]any{
		"placeholder": "подсказка",
		"hint":        "выберите",
		"min_select":  float64(1),
		"max_select":  float64(3),
	})
	if cfg.Placeholder != "подсказка" || cfg.Hint != "выберите" || cfg.MinSelect != 1 || cfg.MaxSelect != 3 {
		t.Errorf("ParseQuestionConfig: %+v", cfg)
	}
}

func TestQuestionConfig_ToMap(t *testing.T) {
	m := (QuestionConfig{Hint: "hint", MaxSelect: 2}).ToMap()
	if m["hint"] != "hint" || m["max_select"] != 2 {
		t.Errorf("ToMap: %+v", m)
	}
	if len(QuestionConfig{}.ToMap()) != 0 {
		t.Error("пустой config должен давать пустую map")
	}
}

func TestNewQuestion_GenresSourceWithoutInlineOptions(t *testing.T) {
	q, err := NewQuestion(1, "Жанр", "tags", "GENRE", true, OptionSourceGenres, QuestionConfig{MaxSelect: 3}, nil)
	if err != nil {
		t.Fatalf("жанры из справочника не требуют inline options: %v", err)
	}
	if q.OptionSource != OptionSourceGenres || q.Config.MaxSelect != 3 {
		t.Errorf("неверный вопрос: %+v", q)
	}
}

func TestNewQuestion_InlineTagsRequireOptions(t *testing.T) {
	_, err := NewQuestion(1, "Жанр", "tags", "GENRE", true, OptionSourceInline, QuestionConfig{}, nil)
	if err == nil {
		t.Error("inline tags без options должны отклоняться")
	}
}

func TestNewQuestion_InvalidOptionSource(t *testing.T) {
	_, err := NewQuestion(1, "Текст", "text", "KEY", true, "unknown", QuestionConfig{}, nil)
	if err == nil {
		t.Error("ожидали ошибку при недопустимом option_source")
	}
}
