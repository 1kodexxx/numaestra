import { describe, expect, it } from "vitest";
import {
  composeCatalogBrief,
  composeCategoryBrief,
  buildCatalogStyleTags,
  buildStyleTagsFromAnswers,
  formatQuizDescription,
  type GenreOption,
} from "./sunoPrompt";

const GENRES: GenreOption[] = [
  { label: "Поп", sunoValue: "modern pop" },
  { label: "Баллада", sunoValue: "pop ballad" },
];

describe("sunoPrompt", () => {
  it("composeCatalogBrief кодирует tags и описание", () => {
    const brief = composeCatalogBrief(
      {
        occasion: "жене на годовщину",
        moods: ["Романтика"],
        genres: ["Поп"],
        tempo: "Медленный",
        vocal: "Мужской",
        details: "15 лет вместе",
        customText: "",
      },
      GENRES,
    );
    expect(brief).toContain("#SUNO_TAGS#");
    expect(brief).toContain("modern pop");
    expect(brief).toContain("male vocals");
    expect(brief).toContain("Vocal requirement");
    expect(brief).toContain("#SUNO_DESC#");
    expect(brief).toContain("15 лет вместе");
  });

  it("buildCatalogStyleTags использует suno_value жанров", () => {
    const tags = buildCatalogStyleTags(
      {
        occasion: "",
        moods: [],
        genres: ["Баллада"],
        tempo: "",
        vocal: "",
        details: "",
        customText: "",
      },
      GENRES,
    );
    expect(tags).toContain("pop ballad");
    expect(tags).not.toContain("Баллада");
  });

  it("composeCategoryBrief кодирует tags и описание как основной конструктор", () => {
    const template =
      "Create a [MOOD] [GENRE] song with [VOCAL]. The lyrics must be in Russian language. Birthday person is [NAME].";
    const brief = composeCategoryBrief("День рождения", template, {
      NAME: "Коля",
      GENRE: "modern pop",
      MOOD: "warm, friendly",
      VOCAL: "female vocals",
    });
    expect(brief).toContain("#SUNO_TAGS#");
    expect(brief).toContain("modern pop");
    expect(brief).toContain("#SUNO_DESC#");
    expect(brief).toContain("Коля");
    expect(brief).not.toContain("Create a");
    expect(brief).toContain("Vocal requirement");
  });

  it("buildStyleTagsFromAnswers ставит вокал первым в tags", () => {
    const tags = buildStyleTagsFromAnswers({
      GENRE: "modern pop",
      VOCAL: "female vocals",
    });
    expect(tags.indexOf("female vocals")).toBeLessThan(tags.indexOf("modern pop"));
  });

  it("buildStyleTagsFromAnswers добавляет russian lyrics один раз", () => {
    const tags = buildStyleTagsFromAnswers({
      GENRE: "modern pop",
      MOOD: "emotional",
      VOCAL: "male vocals",
      TEMPO: "slow tempo",
    });
    expect(tags.match(/russian/gi)?.length).toBe(1);
  });

  it("formatQuizDescription сохраняет extra и убирает бойлерплейт", () => {
    const subst =
      "Create a emotional modern pop song with male vocals. The lyrics must be in Russian language. Groom Ivan and bride Maria.";
    const desc = formatQuizDescription("Свадьба", subst, "фраза «навсегда»");
    expect(desc).toContain("Ivan");
    expect(desc).toContain("навсегда");
    expect(desc).not.toContain("Create a");
  });
});
