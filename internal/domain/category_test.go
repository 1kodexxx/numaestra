package domain

import (
	"encoding/json"
	"testing"
)

func testCategory() *Category {
	return RestoreCategory(CategorySnapshot{
		ID:                 "wedding",
		Title:              "Песня на свадьбу",
		Description:        "Идеальный саундтрек",
		CoverImageURL:      "/images/wedding.jpg",
		SeoTags:            []string{"свадьба", "подарок"},
		BasePromptTemplate: "Create a [MOOD] song about [TOPIC]",
		Questions: []Question{
			{ID: 1, StepNumber: 1, QuestionText: "Как зовут жениха?", UIType: "text", MappingKey: "GROOM", IsRequired: true},
		},
	})
}

// --- RestoreCategory / Snapshot roundtrip ---

func TestRestoreCategory_Roundtrip(t *testing.T) {
	snap := CategorySnapshot{
		ID:                 "corporate",
		Title:              "Гимн компании",
		Description:        "Мощный трек",
		CoverImageURL:      "/images/corp.jpg",
		SeoTags:            []string{"бизнес", "корпоратив"},
		BasePromptTemplate: "Create a [GENRE] anthem",
		Questions: []Question{
			{ID: 2, StepNumber: 1, QuestionText: "Название компании?", UIType: "text", MappingKey: "COMPANY", IsRequired: true,
				Options: []Option{{Label: "A", Value: "a"}}},
		},
	}
	cat := RestoreCategory(snap)
	got := cat.Snapshot()

	if got.ID != snap.ID {
		t.Errorf("ID: got %q, want %q", got.ID, snap.ID)
	}
	if got.Title != snap.Title {
		t.Errorf("Title: got %q, want %q", got.Title, snap.Title)
	}
	if got.Description != snap.Description {
		t.Errorf("Description: got %q, want %q", got.Description, snap.Description)
	}
	if got.CoverImageURL != snap.CoverImageURL {
		t.Errorf("CoverImageURL: got %q, want %q", got.CoverImageURL, snap.CoverImageURL)
	}
	if got.BasePromptTemplate != snap.BasePromptTemplate {
		t.Errorf("BasePromptTemplate: got %q, want %q", got.BasePromptTemplate, snap.BasePromptTemplate)
	}
	if len(got.SeoTags) != len(snap.SeoTags) {
		t.Errorf("SeoTags len: got %d, want %d", len(got.SeoTags), len(snap.SeoTags))
	}
	if len(got.Questions) != len(snap.Questions) {
		t.Errorf("Questions len: got %d, want %d", len(got.Questions), len(snap.Questions))
	}
}

// --- Геттеры ---

func TestCategory_Getters(t *testing.T) {
	c := testCategory()

	if c.ID() != "wedding" {
		t.Errorf("ID: got %q", c.ID())
	}
	if c.Title() != "Песня на свадьбу" {
		t.Errorf("Title: got %q", c.Title())
	}
	if c.Description() != "Идеальный саундтрек" {
		t.Errorf("Description: got %q", c.Description())
	}
	if c.CoverImageURL() != "/images/wedding.jpg" {
		t.Errorf("CoverImageURL: got %q", c.CoverImageURL())
	}
	if c.BasePromptTemplate() != "Create a [MOOD] song about [TOPIC]" {
		t.Errorf("BasePromptTemplate: got %q", c.BasePromptTemplate())
	}
	if len(c.SeoTags()) != 2 {
		t.Errorf("SeoTags len: got %d, want 2", len(c.SeoTags()))
	}
	if len(c.Questions()) != 1 {
		t.Errorf("Questions len: got %d, want 1", len(c.Questions()))
	}
}

// --- MarshalJSON ---

func TestCategory_MarshalJSON_ExcludesBasePromptTemplate(t *testing.T) {
	c := testCategory()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if _, ok := m["base_prompt_template"]; ok {
		t.Error("base_prompt_template не должен попадать в публичный JSON")
	}
	if m["id"] != "wedding" {
		t.Errorf("id: got %v", m["id"])
	}
	if m["title"] != "Песня на свадьбу" {
		t.Errorf("title: got %v", m["title"])
	}
	if m["description"] != "Идеальный саундтрек" {
		t.Errorf("description: got %v", m["description"])
	}
	if m["cover_image_url"] != "/images/wedding.jpg" {
		t.Errorf("cover_image_url: got %v", m["cover_image_url"])
	}
}

func TestCategory_MarshalJSON_IncludesQuestionsWhenPresent(t *testing.T) {
	c := testCategory()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var result struct {
		Questions []Question `json:"questions"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(result.Questions) != 1 {
		t.Errorf("questions: got %d, want 1", len(result.Questions))
	}
}

func TestCategory_MarshalJSON_OmitsEmptyQuestions(t *testing.T) {
	c := RestoreCategory(CategorySnapshot{ID: "minimal", Title: "T", BasePromptTemplate: "P"})
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	if _, ok := m["questions"]; ok {
		t.Error("questions должны быть omitempty при пустом списке")
	}
}
