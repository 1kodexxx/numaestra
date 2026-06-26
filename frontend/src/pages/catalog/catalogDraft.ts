import type { GenreOption } from '@shared/lib/sunoPrompt'
import type { PromptForm } from './types'
import { EMPTY_FORM } from './types'

const KEY_V1 = 'numaestra_catalog_draft_v1'
const KEY_V2 = 'numaestra_catalog_draft_v2'

function draftHasContent(form: PromptForm): boolean {
  return Boolean(
    form.occasion.trim()
    || form.details.trim()
    || form.moods.length > 0
    || form.genres.length > 0
    || form.customGenres.length > 0
    || form.tempo.trim()
    || form.vocal.trim()
    || form.customText.trim(),
  )
}

/** Миграция v1 (labels) → v2 (sunoValue + customGenres). */
function migrateV1(raw: Record<string, unknown>, genres: GenreOption[]): PromptForm {
  const form = { ...EMPTY_FORM, ...(raw as Partial<PromptForm>) }
  const labelToSuno = new Map(genres.map((g) => [g.label, g.sunoValue]))
  const migratedGenres: string[] = []
  const customGenres: string[] = [...(form.customGenres ?? [])]

  for (const entry of form.genres ?? []) {
    const suno = labelToSuno.get(entry)
    if (suno) {
      if (!migratedGenres.includes(suno)) migratedGenres.push(suno)
    } else if (entry.trim() && !customGenres.includes(entry)) {
      customGenres.push(entry)
    }
  }

  return {
    ...form,
    genres: migratedGenres,
    customGenres,
  }
}

export function loadCatalogDraft(genres: GenreOption[] = []): PromptForm | null {
  try {
    const rawV2 = localStorage.getItem(KEY_V2)
    if (rawV2) {
      const form = JSON.parse(rawV2) as PromptForm
      const normalized: PromptForm = {
        ...EMPTY_FORM,
        ...form,
        customGenres: form.customGenres ?? [],
      }
      return draftHasContent(normalized) ? normalized : null
    }

    const rawV1 = localStorage.getItem(KEY_V1)
    if (!rawV1) return null
    const parsed = JSON.parse(rawV1) as Record<string, unknown>
    const form = migrateV1(parsed, genres)
    return draftHasContent(form) ? form : null
  } catch {
    return null
  }
}

export function hasCatalogDraft(genres: GenreOption[] = []): boolean {
  return loadCatalogDraft(genres) !== null
}

export function saveCatalogDraft(form: PromptForm): void {
  try {
    if (!draftHasContent(form)) {
      localStorage.removeItem(KEY_V2)
      localStorage.removeItem(KEY_V1)
      return
    }
    localStorage.setItem(KEY_V2, JSON.stringify(form))
    localStorage.removeItem(KEY_V1)
  } catch {
    /* quota / private mode */
  }
}

export function clearCatalogDraft(): void {
  try {
    localStorage.removeItem(KEY_V2)
    localStorage.removeItem(KEY_V1)
  } catch {
    /* ignore */
  }
}
