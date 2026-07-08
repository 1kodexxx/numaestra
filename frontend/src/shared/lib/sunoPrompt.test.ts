import { describe, expect, it } from "vitest";
import {
  buildCatalogDescription,
  buildCatalogStyleTags,
  buildStyleTagsFromAnswers,
  composeCatalogBrief,
  composeCategoryBrief,
  extractCustomGenresFromAnswers,
  formatQuizDescription,
  stripUnfilledPlaceholders,
  encodeSunoPrompt,
  isCompleteSongLyrics,
  SUNO_LYRICS_MARKER,
  type GenreOption,
} from "./sunoPrompt";

const GENRES: GenreOption[] = [
  { label: "Поп", sunoValue: "modern pop" },
  { label: "Баллада", sunoValue: "pop ballad" },
];

describe("sunoPrompt", () => {
  it("stripUnfilledPlaceholders убирает предложение с пропущенным [KEY]", () => {
    expect(
      stripUnfilledPlaceholders(
        "Celebrating 5 years together. How they met: [MEET_STORY].",
      ),
    ).toBe("Celebrating 5 years together.");
  });

  it("stripUnfilledPlaceholders не трогает полностью заполненный текст", () => {
    const filled = "Celebrating 5 years together. How they met: at university.";
    expect(stripUnfilledPlaceholders(filled)).toBe(filled);
  });

  it("encodeSunoPrompt кладёт готовый текст в отдельный канал #SUNO_LYRICS#", () => {
    const out = encodeSunoPrompt(
      "rap, male vocals",
      "идея про город",
      "[Verse]\nмой текст\n[Chorus]\nприпев",
    );
    expect(out).toContain(SUNO_LYRICS_MARKER);
    expect(out).toContain("мой текст");
    // Текст не должен дублироваться внутри описания старой строкой «Must-use lyrics».
    expect(out).not.toContain("Must-use lyrics");
  });

  it("composeCatalogBrief отправляет customText как lyrics, не в описание", () => {
    const brief = composeCatalogBrief(
      {
        occasion: "просто трек",
        moods: [],
        genres: ["modern pop"],
        customGenres: [],
        tempo: "Средний",
        vocal: "Мужской",
        details: "",
        customText: "[Verse]\nГде-то под утро висим",
      },
      GENRES,
    );
    expect(brief).toContain(SUNO_LYRICS_MARKER);
    expect(brief).toContain("Где-то под утро висим");
    expect(brief).not.toContain("Must-use lyrics");
  });

  it("composeCatalogBrief кодирует tags и описание", () => {
    const brief = composeCatalogBrief(
      {
        occasion: "жене на годовщину",
        moods: ["Романтика"],
        genres: ["modern pop"],
        customGenres: [],
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
        genres: ["pop ballad"],
        customGenres: [],
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

  it("Без вокала — instrumental в tags, без russian lyrics и vocal requirement", () => {
    const form = {
      occasion: "фон для видео",
      moods: [],
      genres: ["modern pop"],
      customGenres: [],
      tempo: "",
      vocal: "Без вокала",
      details: "спокойная мелодия",
      customText: "",
    };
    const tags = buildCatalogStyleTags(form, GENRES);
    expect(tags).toContain("instrumental");
    expect(tags).not.toMatch(/russian lyrics/i);

    const desc = buildCatalogDescription(form);
    expect(desc).not.toMatch(/must be in Russian/i);
    expect(desc).toContain("Instrumental track only");
    expect(desc).not.toContain("Vocal requirement");
  });

  it("неизвестный label не попадает в tags как кириллица", () => {
    const tags = buildCatalogStyleTags(
      {
        occasion: "",
        moods: [],
        genres: ["unknown-value"],
        customGenres: ["Трэп"],
        tempo: "",
        vocal: "",
        details: "",
        customText: "",
      },
      GENRES,
    );
    expect(tags).not.toContain("Трэп");
    expect(tags).not.toContain("unknown-value");
  });

  it("custom genre попадает в description категории, не в tags", () => {
    const answers = {
      GENRE: "modern pop, Трэп",
      VOCAL: "male vocals",
    };
    const tags = buildStyleTagsFromAnswers(answers);
    expect(tags).not.toContain("Трэп");
    expect(extractCustomGenresFromAnswers(answers)).toEqual(["Трэп"]);

    const brief = composeCategoryBrief(
      "День рождения",
      "Song about [NAME]",
      { ...answers, NAME: "Коля" },
    );
    expect(brief).toContain("Preferred genre style: Трэп");
    expect(brief).not.toContain("#SUNO_TAGS# Трэп");
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
    expect(tags.indexOf("female vocals")).toBeLessThan(
      tags.indexOf("modern pop"),
    );
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

  // Зеркало pkg/suno IsCompleteSongLyrics: короткая фраза — не полный текст
  // (кейс «с любовью к папе» → 30-секундный обрубок в Custom Mode).
  it("isCompleteSongLyrics отличает фразу от полного текста", () => {
    expect(isCompleteSongLyrics("")).toBe(false);
    expect(isCompleteSongLyrics("с любовью к папе")).toBe(false);
    expect(isCompleteSongLyrics("Папа, ты мой герой\nПапа, ты всегда со мной")).toBe(false);
    expect(isCompleteSongLyrics("[Verse]\nСтрока\n[Chorus]\nПрипев")).toBe(true);
    expect(isCompleteSongLyrics("[Куплет]\nСтрока")).toBe(true);
    const line = "Полноценная строка текста песни о самом дорогом человеке";
    expect(isCompleteSongLyrics((line + "\n").repeat(10))).toBe(true);
    expect(isCompleteSongLyrics((line + " ").repeat(10))).toBe(false);
  });
});
