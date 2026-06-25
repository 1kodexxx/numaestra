import { describe, expect, it } from "vitest";
import {
  composeCatalogBrief,
  buildCatalogStyleTags,
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
});
