import { apiFetch } from '@shared/api'
import type { AdminReview } from './types'

export const adminReviewApi = {
  list() {
    return apiFetch<AdminReview[]>('/admin/reviews/')
  },

  reply(id: string, message: string) {
    return apiFetch<AdminReview>(`/admin/reviews/${id}/reply`, { method: 'POST', body: { message } })
  },

  setPublished(id: string, isPublished: boolean) {
    return apiFetch<AdminReview>(`/admin/reviews/${id}`, { method: 'PATCH', body: { is_published: isPublished } })
  },

  remove(id: string) {
    return apiFetch<void>(`/admin/reviews/${id}`, { method: 'DELETE' })
  },
}
