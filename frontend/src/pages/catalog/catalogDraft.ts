import type { PromptForm } from './types'

const KEY = 'numaestra_catalog_draft_v1'

export function loadCatalogDraft(): PromptForm | null {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return null
    return JSON.parse(raw) as PromptForm
  } catch {
    return null
  }
}

export function saveCatalogDraft(form: PromptForm): void {
  try {
    const hasContent = form.occasion.trim() || form.details.trim() || form.moods.length > 0
    if (!hasContent) {
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
