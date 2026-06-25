/** Suno-friendly prompt encoding shared by catalog constructor. */

export const SUNO_TAGS_MARKER = "#SUNO_TAGS#";
export const SUNO_DESC_MARKER = "#SUNO_DESC#";

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
  genres: string[];
  tempo: string;
  vocal: string;
  details: string;
  customText: string;
}

function uniqueTags(parts: string[]): string {
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
  if (!out.some((t) => t.toLowerCase().includes("russian"))) {
    out.push("russian lyrics");
  }
  return out.join(", ");
}

export function buildCatalogStyleTags(
  form: CatalogPromptForm,
  genreOptions: GenreOption[],
): string {
  const labelToSuno = new Map(genreOptions.map((g) => [g.label, g.sunoValue]));
  const parts: string[] = [];

  for (const label of form.genres) {
    parts.push(labelToSuno.get(label) ?? label);
  }
  for (const mood of form.moods) {
    parts.push(MOOD_SUNO[mood] ?? mood);
  }
  if (form.tempo) parts.push(TEMPO_SUNO[form.tempo] ?? form.tempo);
  if (form.vocal) parts.push(VOCAL_SUNO[form.vocal] ?? form.vocal);

  return uniqueTags(parts);
}

export function buildCatalogDescription(form: CatalogPromptForm): string {
  const lines: string[] = [
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
  if (form.customText.trim()) {
    lines.push(
      "",
      "Must-use lyrics (include verbatim where possible):",
      form.customText.trim(),
    );
  }
  lines.push(
    "",
    "Match the musical style from the separate Suno style tags (genre, mood, vocals, tempo).",
    "Make the chorus catchy and personal.",
  );
  return lines.join("\n");
}

export function encodeSunoPrompt(tags: string, description: string): string {
  const t = tags.trim();
  const d = description.trim();
  if (!t && !d) return "";
  let out = "";
  if (t) out += `${SUNO_TAGS_MARKER} ${t}\n`;
  out += `${SUNO_DESC_MARKER}\n${d}`;
  return out;
}

export function composeCatalogBrief(
  form: CatalogPromptForm,
  genreOptions: GenreOption[],
): string {
  return encodeSunoPrompt(
    buildCatalogStyleTags(form, genreOptions),
    buildCatalogDescription(form),
  );
}
