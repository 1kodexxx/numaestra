import { apiFetch } from '@shared/api'
import type { Genre } from './types'

export const genreApi = {
  list(categoryId?: string) {
    const q = categoryId ? `?category_id=${encodeURIComponent(categoryId)}` : ''
    return apiFetch<Genre[]>(`/genres${q}`)
  },
}
