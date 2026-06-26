import type { PromptForm } from './types'

const KEY = 'numaestra_catalog_draft_v1'

function draftHasContent(form: PromptForm): boolean {
  return Boolean(
    form.occasion.trim()
    || form.details.trim()
    || form.moods.length > 0
    || form.genres.length > 0
    || form.customText.trim(),
  )
}

export function loadCatalogDraft(): PromptForm | null {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return null
    const form = JSON.parse(raw) as PromptForm
    return draftHasContent(form) ? form : null
  } catch {
    return null
  }
}

export function hasCatalogDraft(): boolean {
  return loadCatalogDraft() !== null
}

export function saveCatalogDraft(form: PromptForm): void {
  try {
    if (!draftHasContent(form)) {
      localStorage.removeItem(KEY)
      return
    }
    localStorage.setItem(KEY, JSON.stringify(form))
  } catch {
    /* quota / private mode */
  }
}

export function clearCatalogDraft(): void {
  try {
    localStorage.removeItem(KEY)
  } catch {
    /* ignore */
  }
}
