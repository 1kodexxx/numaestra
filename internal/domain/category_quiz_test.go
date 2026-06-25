package domain

import (
	"errors"
	"testing"
)

func TestValidateQuizAnswers_RequiredMissing(t *testing.T) {
	questions := []Question{
		{MappingKey: "OCCASION", QuestionText: "Повод", IsRequired: true},
		{MappingKey: "MOOD", QuestionText: "Настроение", IsRequired: false},
	}
	err := ValidateQuizAnswers(questions, map[string]string{"MOOD": "радость"})
	if !errors.Is(err, ErrMissingQuizAnswers) {
		t.Fatalf("ожидали ErrMissingQuizAnswers, получили %v", err)
	}
}

func TestValidateQuizAnswers_AllRequiredPresent(t *testing.T) {
	questions := []Question{
		{MappingKey: "OCCASION", QuestionText: "Повод", IsRequired: true},
	}
	if err := ValidateQuizAnswers(questions, map[string]string{"OCCASION": " юбилей "}); err != nil {
		t.Fatalf("ожидали успех, получили %v", err)
	}
}
