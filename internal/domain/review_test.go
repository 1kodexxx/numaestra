package domain

import (
	"strings"
	"testing"
)

func TestNewReview_Validation(t *testing.T) {
	cases := []struct {
		name    string
		author  string
		rating  int
		body    string
		wantErr bool
	}{
		{"ok", "Иван", 5, "Отличный сервис", false},
		{"trims and ok", "  Аня  ", 4, "  текст  ", false},
		{"empty author", "", 5, "текст", true},
		{"empty body", "Иван", 5, "   ", true},
		{"rating too low", "Иван", 0, "текст", true},
		{"rating too high", "Иван", 6, "текст", true},
		{"author too long", strings.Repeat("я", 81), 5, "текст", true},
		{"body too long", "Иван", 5, strings.Repeat("a", 2001), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewReview(c.author, c.rating, c.body)
			if (err != nil) != c.wantErr {
				t.Errorf("NewReview(%q,%d): wantErr=%v, got %v", c.author, c.rating, c.wantErr, err)
			}
		})
	}
}

func TestReview_SetAdminReply_TogglesTimestamp(t *testing.T) {
	r, _ := NewReview("Иван", 5, "текст")

	if err := r.SetAdminReply("Спасибо!"); err != nil {
		t.Fatalf("SetAdminReply: %v", err)
	}
	if r.AdminReply() != "Спасибо!" || r.AdminReplyAt() == nil {
		t.Error("ответ и время должны быть установлены")
	}

	// Пустой ответ снимает отметку времени.
	_ = r.SetAdminReply("")
	if r.AdminReply() != "" || r.AdminReplyAt() != nil {
		t.Error("пустой ответ должен сбросить ответ и время")
	}

	if err := r.SetAdminReply(strings.Repeat("a", 2001)); err == nil {
		t.Error("слишком длинный ответ должен отвергаться")
	}
}

func TestReview_SetPublished(t *testing.T) {
	r, _ := NewReview("Иван", 5, "текст")
	if !r.IsPublished() {
		t.Error("новый отзыв публикуется сразу")
	}
	r.SetPublished(false)
	if r.IsPublished() {
		t.Error("SetPublished(false) должен скрыть отзыв")
	}
}
