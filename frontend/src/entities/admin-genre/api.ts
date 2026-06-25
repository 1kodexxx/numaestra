import { apiFetch } from '@shared/api'
import type { AdminGenre, GenrePayload, GenreUpdatePayload } from './types'

export const adminGenreApi = {
  list() {
    return apiFetch<AdminGenre[]>('/admin/genres/')
  },

  create(payload: GenrePayload) {
    return apiFetch<AdminGenre>('/admin/genres/', { method: 'POST', body: payload })
  },

  update(id: number, payload: GenreUpdatePayload) {
    return apiFetch<AdminGenre>(`/admin/genres/${id}`, { method: 'PUT', body: payload })
  },

  remove(id: number) {
    return apiFetch<void>(`/admin/genres/${id}`, { method: 'DELETE' })
  },

  setCategoryGenres(categoryId: string, genreIds: number[]) {
    return apiFetch<void>(`/admin/categories/${categoryId}/genres`, {
      method: 'PUT',
      body: { genre_ids: genreIds },
    })
  },
}
