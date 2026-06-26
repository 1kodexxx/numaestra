package domain

import (
	"encoding/json"
	"testing"
	"time"
)

// --- Example tests ---

func TestNewExample_Valid(t *testing.T) {
	e, err := NewExample("slug-1", "Название", "pop", "описание", "happy", "http://audio", "http://cover", 1, true)
	if err != nil {
		t.Fatalf("ожидали успех: %v", err)
	}
	if e.ID() != "slug-1" {
		t.Errorf("неверный ID: %q", e.ID())
	}
	if e.Title() != "Название" {
		t.Errorf("неверный Title: %q", e.Title())
	}
	if e.Category() != "pop" {
		t.Errorf("неверный Category: %q", e.Category())
	}
	if e.Description() != "описание" {
		t.Errorf("неверный Description: %q", e.Description())
	}
	if e.Mood() != "happy" {
		t.Errorf("неверный Mood: %q", e.Mood())
	}
	if e.AudioURL() != "http://audio" {
		t.Errorf("неверный AudioURL: %q", e.AudioURL())
	}
	if e.CoverURL() != "http://cover" {
		t.Errorf("неверный CoverURL: %q", e.CoverURL())
	}
	if e.SortOrder() != 1 {
		t.Errorf("неверный SortOrder: %d", e.SortOrder())
	}
	if !e.IsActive() {
		t.Error("должен быть активен")
	}
}

func TestNewExample_EmptyID(t *testing.T) {
	_, err := NewExample("", "Название", "", "", "", "", "", 0, true)
	if err == nil {
		t.Fatal("ожидали ошибку для пустого ID")
	}
}

func TestNewExample_EmptyTitle(t *testing.T) {
	_, err := NewExample("id", "", "", "", "", "", "", 0, true)
	if err == nil {
		t.Fatal("ожидали ошибку для пустого Title")
	}
}

func TestExample_UpdateDetails(t *testing.T) {
	e, _ := NewExample("id", "Старый", "cat", "", "", "", "", 0, true)
	err := e.UpdateDetails("Новый", "pop", "desc", "sad", "http://a", "http://c", 5, false)
	if err != nil {
		t.Fatalf("UpdateDetails: %v", err)
	}
	if e.Title() != "Новый" {
		t.Errorf("неверный Title: %q", e.Title())
	}
	if e.SortOrder() != 5 {
		t.Errorf("неверный SortOrder: %d", e.SortOrder())
	}
	if e.IsActive() {
		t.Error("должен быть неактивен")
	}
}

func TestExample_UpdateDetails_EmptyTitle(t *testing.T) {
	e, _ := NewExample("id", "Старый", "", "", "", "", "", 0, true)
	if err := e.UpdateDetails("", "", "", "", "", "", 0, true); err == nil {
		t.Fatal("ожидали ошибку для пустого Title")
	}
}

func TestExample_SnapshotRestore(t *testing.T) {
	e, _ := NewExample("snap-id", "Снапшот", "rock", "desc", "energetic", "http://a", "http://c", 3, true)
	snap := e.Snapshot()
	if snap.ID != "snap-id" || snap.Title != "Снапшот" || snap.SortOrder != 3 {
		t.Errorf("неверный снапшот: %+v", snap)
	}

	restored := RestoreExample(snap)
	if restored.ID() != "snap-id" {
		t.Errorf("неверный ID после восстановления: %q", restored.ID())
	}
	if restored.Category() != "rock" {
		t.Errorf("неверный Category после восстановления: %q", restored.Category())
	}
}

func TestExample_MarshalJSON(t *testing.T) {
	e, _ := NewExample("json-id", "JSON тест", "pop", "desc", "mood", "http://audio", "http://cover", 2, true)
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if m["id"] != "json-id" {
		t.Errorf("неверный id в JSON: %v", m["id"])
	}
	if m["is_active"] != true {
		t.Errorf("неверный is_active в JSON: %v", m["is_active"])
	}
	if m["audio_url"] != "http://audio" {
		t.Errorf("неверный audio_url в JSON: %v", m["audio_url"])
	}
}

// --- Review tests (RestoreReview, Snapshot, accessors) ---

func TestReview_RestoreAndSnapshot(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	snap := ReviewSnapshot{
		AuthorName:   "Иван",
		Rating:       5,
		Body:         "Отличная песня!",
		AdminReply:   "Спасибо!",
		AdminReplyAt: func() *time.Time { t2 := now; return &t2 }(),
		IsPublished:  true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	r := RestoreReview(snap)

	if r.AuthorName() != "Иван" {
		t.Errorf("AuthorName: %q", r.AuthorName())
	}
	if r.Rating() != 5 {
		t.Errorf("Rating: %d", r.Rating())
	}
	if r.Body() != "Отличная песня!" {
		t.Errorf("Body: %q", r.Body())
	}
	if r.AdminReply() != "Спасибо!" {
		t.Errorf("AdminReply: %q", r.AdminReply())
	}
	if r.AdminReplyAt() == nil {
		t.Error("AdminReplyAt должен быть установлен")
	}
	if !r.IsPublished() {
		t.Error("IsPublished должен быть true")
	}
	if r.CreatedAt().IsZero() {
		t.Error("CreatedAt не должен быть нулевым")
	}

	snap2 := r.Snapshot()
	if snap2.AuthorName != "Иван" || snap2.Rating != 5 {
		t.Errorf("Snapshot неверный: %+v", snap2)
	}
	if snap2.AdminReply != "Спасибо!" {
		t.Errorf("AdminReply в снапшоте: %q", snap2.AdminReply)
	}
}

func TestReview_Accessors_ID(t *testing.T) {
	snap := ReviewSnapshot{
		AuthorName: "Тест",
		Rating:     4,
		Body:       "Текст",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	r := RestoreReview(snap)
	// ID — нулевой uuid если не передан
	_ = r.ID()
}

// --- Category tests ---

func TestNewCategory_Valid(t *testing.T) {
	cat, err := NewCategory("wedding", "Свадьба", "для свадебного торжества", "/img.svg", []string{"свадьба"}, "Create a song: [NAME]")
	if err != nil {
		t.Fatalf("ожидали успех: %v", err)
	}
	if cat.ID() != "wedding" {
		t.Errorf("неверный ID: %q", cat.ID())
	}
	if cat.Title() != "Свадьба" {
		t.Errorf("неверный Title: %q", cat.Title())
	}
}

func TestNewCategory_Validation(t *testing.T) {
	cases := []struct{ id, title, tmpl string }{
		{"", "Тест", "tmpl"},
		{"id", "", "tmpl"},
		{"id", "Тест", ""},
	}
	for _, c := range cases {
		if _, err := NewCategory(c.id, c.title, "", "", nil, c.tmpl); err == nil {
			t.Errorf("ожидали ошибку для id=%q title=%q tmpl=%q", c.id, c.title, c.tmpl)
		}
	}
}

func TestCategory_UpdateDetails(t *testing.T) {
	cat, _ := NewCategory("id", "Старый", "", "", nil, "tmpl1")
	if err := cat.UpdateDetails("Новый", "новое описание", "/cover.svg", []string{"tag"}, "tmpl2"); err != nil {
		t.Fatalf("UpdateDetails: %v", err)
	}
	if cat.Title() != "Новый" {
		t.Errorf("неверный Title: %q", cat.Title())
	}
}

func TestCategory_UpdateDetails_Validation(t *testing.T) {
	cat, _ := NewCategory("id", "Тест", "", "", nil, "tmpl")
	if err := cat.UpdateDetails("", "", "", nil, "tmpl"); err == nil {
		t.Error("ожидали ошибку для пустого title")
	}
	if err := cat.UpdateDetails("Тест", "", "", nil, ""); err == nil {
		t.Error("ожидали ошибку для пустого template")
	}
}
