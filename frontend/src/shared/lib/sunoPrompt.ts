/** Suno-friendly prompt encoding shared by catalog constructor. */

export const SUNO_TAGS_MARKER = "#SUNO_TAGS#";
export const SUNO_DESC_MARKER = "#SUNO_DESC#";
export const SUNO_LYRICS_MARKER = "#SUNO_LYRICS#";

// Порог «полного текста песни» — зеркало бэкенда (pkg/suno/prompt.go,
// IsCompleteSongLyrics). Короче порога — сервер вплетёт строки в песню
// дословно, а остальной текст допишет нейросеть (Inspiration Mode).
const MIN_COMPLETE_LYRICS_CHARS = 400;
const MIN_COMPLETE_LYRICS_LINES = 8;

/** Текст тянет на полную песню для Custom Mode (Suno поёт только его). */
export function isCompleteSongLyrics(s: string): boolean {
  const t = s.trim();
  if (!t) return false;
  const lower = t.toLowerCase();
  if (
    lower.includes("[verse") ||
    lower.includes("[chorus") ||
    lower.includes("[куплет") ||
    lower.includes("[припев")
  ) {
    return true;
  }
  const lines = t.split("\n").filter((l) => l.trim() !== "").length;
  return [...t].length >= MIN_COMPLETE_LYRICS_CHARS && lines >= MIN_COMPLETE_LYRICS_LINES;
}

export interface GenreOption {
  label: string;
  sunoValue: string;
}

const MOOD_SUNO: Record<string, string> = {
  Романтика: "romantic, heartfelt",
  Радость: "joyful, uplifting",
  Грусть: "melancholic, emotional",
  Ностальгия: "nostalgic, warm",
  Энергия: "energetic, vibrant",
  Торжественность: "ceremonial, majestic",
  Юмор: "funny, playful, comedic",
  Спокойствие: "calm, peaceful, soft",
  Драйв: "driving, high energy",
};

const TEMPO_SUNO: Record<string, string> = {
  Медленный: "slow tempo",
  Средний: "medium tempo",
  Быстрый: "upbeat fast tempo",
};

const VOCAL_SUNO: Record<string, string> = {
  Мужской: "male vocals",
  Женский: "female vocals",
  Дуэт: "male and female duet vocals",
  Хор: "choir vocals",
  "Без вокала": "instrumental",
};

export interface CatalogPromptForm {
  occasion: string;
  moods: string[];
  /** sunoValue пресетов из API */
  genres: string[];
  /** пользовательские жанры — только в description, не в tags */
  customGenres: string[];
  tempo: string;
  vocal: string;
  details: string;
  customText: string;
}

/** true, если в tags есть instrumental как отдельный тег. */
export function isInstrumentalFromTags(tags: string): boolean {
  return tags
    .split(",")
    .map((p) => p.trim().toLowerCase())
    .some((p) => p === "instrumental");
}

export function isInstrumentalVocal(vocal: string): boolean {
  const v = vocal.trim().toLowerCase();
  return v === "без вокала" || v === "instrumental" || v.includes("instrumental");
}

function containsCyrillic(s: string): boolean {
  return /[\u0400-\u04FF]/.test(s);
}

function uniqueTags(parts: string[], instrumental: boolean): string {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const p of parts) {
    const t = p.trim();
    if (!t) continue;
    const key = t.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(t);
  }
  if (
    !instrumental &&
    !out.some((t) => t.toLowerCase().includes("russian"))
  ) {
    out.push("russian lyrics");
  }
  return out.join(", ");
}

export function buildCatalogStyleTags(
  form: CatalogPromptForm,
  genreOptions: GenreOption[],
): string {
  const knownSuno = new Set(genreOptions.map((g) => g.sunoValue));
  const parts: string[] = [];
  const instrumental =
    isInstrumentalVocal(form.vocal) ||
    isInstrumentalFromTags(VOCAL_SUNO[form.vocal] ?? form.vocal);

  if (form.vocal) parts.push(VOCAL_SUNO[form.vocal] ?? form.vocal);
  for (const val of form.genres) {
    if (knownSuno.has(val) && !containsCyrillic(val)) {
      parts.push(val);
    }
  }
  for (const mood of form.moods) {
    if (!containsCyrillic(mood)) {
      parts.push(MOOD_SUNO[mood] ?? mood);
    }
  }
  if (form.tempo) parts.push(TEMPO_SUNO[form.tempo] ?? form.tempo);

  return uniqueTags(parts, instrumental);
}

export function vocalRequirementLine(vocal: string): string {
  const v = vocal.trim();
  if (!v || isInstrumentalVocal(v)) return "";
  return `\n\nVocal requirement (mandatory, do not change): ${v}.`;
}

export function buildCatalogDescription(form: CatalogPromptForm): string {
  const instrumental = isInstrumentalVocal(form.vocal);
  const lines: string[] = instrumental
    ? [
        "Write a complete personalized instrumental track. Instrumental track only. No vocals.",
        "",
        "What the customer wants to hear:",
      ]
    : [
        "Write a complete personalized song. All sung lyrics must be in Russian.",
        "",
        "What the customer wants to hear:",
      ];
  if (form.occasion.trim()) {
    lines.push(`- Occasion / who it is for: ${form.occasion.trim()}`);
  }
  if (form.details.trim()) {
    lines.push(`- Story and details: ${form.details.trim()}`);
  }
  for (const g of form.customGenres) {
    lines.push(`- Preferred genre style: ${g}`);
  }
  // form.customText (готовый текст клиента) больше НЕ вшивается в описание —
  // он уходит в отдельный канал #SUNO_LYRICS# (Custom Mode), см. composeCatalogBrief.
  if (form.vocal && !instrumental) {
    lines.push(vocalRequirementLine(VOCAL_SUNO[form.vocal] ?? form.vocal));
  }
  lines.push(
    "",
    "Match the musical style from the separate Suno style tags (genre, mood, vocals, tempo).",
    instrumental
      ? "Focus on memorable melody and emotional arc without any singing."
      : "Make the chorus catchy and personal.",
  );
  return lines.join("\n");
}

export function encodeSunoPrompt(
  tags: string,
  description: string,
  lyrics = "",
): string {
  const t = tags.trim();
  const d = description.trim();
  const l = lyrics.trim();
  if (!t && !d && !l) return "";
  let out = "";
  if (t) out += `${SUNO_TAGS_MARKER} ${t}\n`;
  // Готовый текст клиента → Custom Mode (Suno поёт его дословно). Иначе — описание
  // в Inspiration Mode (Suno пишет текст сам). Поведение зеркалит бэкенд (prompt.go).
  if (l) {
    if (d) out += `${SUNO_DESC_MARKER}\n${d}\n`;
    out += `${SUNO_LYRICS_MARKER}\n${l}`;
  } else {
    out += `${SUNO_DESC_MARKER}\n${d}`;
  }
  return out;
}

export function composeCatalogBrief(
  form: CatalogPromptForm,
  genreOptions: GenreOption[],
): string {
  // Без вокала текст игнорируем (instrumental).
  const instrumental = isInstrumentalVocal(form.vocal);
  const lyrics = instrumental ? "" : form.customText.trim();
  return encodeSunoPrompt(
    buildCatalogStyleTags(form, genreOptions),
    buildCatalogDescription(form),
    lyrics,
  );
}

const STYLE_ANSWER_KEYS = ["VOCAL", "GENRE", "MOOD", "TEMPO"] as const;

function splitTagPieces(s: string): string[] {
  return s
    .replace(/;/g, ",")
    .split(",")
    .map((p) => p.trim())
    .filter(Boolean);
}

/** Suno tags из ответов квиза; кириллица в GENRE не попадает в tags. */
export function buildStyleTagsFromAnswers(
  answers: Record<string, string>,
): string {
  const parts: string[] = [];
  const seen = new Set<string>();
  let instrumental = false;

  for (const key of STYLE_ANSWER_KEYS) {
    const v = answers[key]?.trim();
    if (!v) continue;
    for (const piece of splitTagPieces(v)) {
      if (key === "GENRE" && containsCyrillic(piece)) continue;
      const low = piece.toLowerCase();
      if (seen.has(low)) continue;
      seen.add(low);
      parts.push(piece);
      if (low === "instrumental" || low.includes("instrumental")) {
        instrumental = true;
      }
    }
  }

  if (parts.length === 0) {
    return "russian pop, heartfelt";
  }
  if (!instrumental && !parts.some((p) => p.toLowerCase().includes("russian"))) {
    parts.push("russian lyrics");
  }
  return parts.join(", ");
}

/** Жанры с кириллицей из GENRE — для description. */
export function extractCustomGenresFromAnswers(
  answers: Record<string, string>,
): string[] {
  const v = answers.GENRE?.trim();
  if (!v) return [];
  return splitTagPieces(v).filter(containsCyrillic);
}

function cleanSubstitutedTemplate(s: string): string {
  let out = s.trim();
  if (!out) return "";
  out = out.replace(/^create\s+a\s+.+?(?:\.\s*)/is, "");
  out = out.replace(
    /\s*the\s+lyrics\s+must\s+be\s+in\s+russian\s*(?:language)?\.?\s*/gi,
    "",
  );
  out = out.replace(/\s*lyrics\s+in\s+russian\.?\s*/gi, "");
  out = out.replace(/\s*tempo\s+feel:\s*[^.]+\.?\s*/gi, "");
  out = out.replace(
    /\s*optional\s+extra\s+details\s+for\s+lyrics:\s*.+$/gi,
    "",
  );
  return out.trim();
}

const PLACEHOLDER_RE = /\[[A-Z][A-Z0-9_]*\]/;

/**
 * Убирает предложения с неподставленными плейсхолдерами [KEY] — необязательные
 * вопросы, которые клиент пропустил. Зеркало StripUnfilledPlaceholders на бэке,
 * чтобы превью совпадало с тем, что реально уходит в Suno.
 */
export function stripUnfilledPlaceholders(s: string): string {
  if (!PLACEHOLDER_RE.test(s)) return s;
  const kept = s
    .split(". ")
    .filter((p) => !PLACEHOLDER_RE.test(p) && p.trim() !== "");
  let res = kept.join(". ").trim();
  if (res && !/[.!?]$/.test(res)) res += ".";
  return res;
}

export function substituteCategoryTemplate(
  template: string,
  answers: Record<string, string>,
): string {
  const skipKeys = new Set([
    "VOCAL",
    "GENRE",
    "MOOD",
    "TEMPO",
    "EXTRA",
    "CUSTOM_LYRICS",
  ]);
  let result = template;
  for (const [key, value] of Object.entries(answers)) {
    if (skipKeys.has(key)) continue;
    result = result.split(`[${key}]`).join(value);
  }
  return stripUnfilledPlaceholders(result);
}

export function formatQuizDescription(
  categoryTitle: string,
  substituted: string,
  extra: string,
  instrumental = false,
): string {
  const body = cleanSubstitutedTemplate(substituted);
  const lines: string[] = instrumental
    ? [
        "Write a complete instrumental track. Instrumental track only. No vocals.",
        "",
      ]
    : ["Write a complete song. All sung lyrics must be in Russian.", ""];
  const title = categoryTitle.trim();
  if (title) {
    lines.push(`Occasion / song type: ${title}.`, "");
  }
  lines.push(
    instrumental
      ? "What the customer wants to hear (use these facts for mood and theme):"
      : "What the customer wants to hear (use these facts in the lyrics):",
  );
  lines.push(body || substituted);
  if (extra.trim()) {
    lines.push("", "Must-include names, phrases, or details:", extra.trim());
  }
  lines.push(
    "",
    instrumental
      ? "Match the musical style from the separate Suno style tags (genre, mood, vocals, tempo). Focus on memorable melody and emotional arc without any singing."
      : "Match the musical style from the separate Suno style tags (genre, mood, vocals, tempo). Make the chorus memorable and emotionally clear.",
  );
  return lines.join("\n");
}

/** Итоговый промпт категории — тот же формат, что BuildFinalPrompt на бэкенде. */
export function composeCategoryBrief(
  categoryTitle: string,
  basePromptTemplate: string,
  answers: Record<string, string>,
  customText = "",
): string {
  const substituted = substituteCategoryTemplate(
    basePromptTemplate,
    answers,
  );
  const tags = buildStyleTagsFromAnswers(answers);
  const instrumental =
    isInstrumentalFromTags(tags) || isInstrumentalVocal(answers.VOCAL ?? "");
  let extra = answers.EXTRA ?? "";
  for (const g of extractCustomGenresFromAnswers(answers)) {
    const line = `Preferred genre style: ${g}`;
    extra = extra.trim() ? `${extra.trim()}\n${line}` : line;
  }
  let description = formatQuizDescription(
    categoryTitle,
    substituted,
    extra,
    instrumental,
  );
  if (answers.VOCAL?.trim() && !instrumental) {
    description += vocalRequirementLine(answers.VOCAL);
  }
  // Готовый текст клиента → отдельный канал #SUNO_LYRICS# (Custom Mode), не в описание.
  const lyrics = instrumental ? "" : customText.trim();
  return encodeSunoPrompt(tags, description, lyrics);
}
