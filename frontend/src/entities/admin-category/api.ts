import { apiFetch } from '@shared/api'
import type { AdminCategory, AdminQuestion, CategoryPayload, QuestionPayload } from './types'

export const adminCategoryApi = {
  list() {
    return apiFetch<AdminCategory[]>('/admin/categories/')
  },

  get(id: string) {
    return apiFetch<AdminCategory>(`/admin/categories/${id}`)
  },

  create(payload: CategoryPayload) {
    return apiFetch<AdminCategory>('/admin/categories/', { method: 'POST', body: payload })
  },

  update(id: string, payload: Omit<CategoryPayload, 'id'>) {
    return apiFetch<AdminCategory>(`/admin/categories/${id}`, { method: 'PUT', body: payload })
  },

  remove(id: string) {
    return apiFetch<void>(`/admin/categories/${id}`, { method: 'DELETE' })
  },

  addQuestion(categoryId: string, payload: QuestionPayload) {
    return apiFetch<AdminQuestion>(`/admin/categories/${categoryId}/questions`, { method: 'POST', body: payload })
  },

  updateQuestion(categoryId: string, questionId: number, payload: QuestionPayload) {
    return apiFetch<void>(`/admin/categories/${categoryId}/questions/${questionId}`, { method: 'PUT', body: payload })
  },

  removeQuestion(categoryId: string, questionId: number) {
    return apiFetch<void>(`/admin/categories/${categoryId}/questions/${questionId}`, { method: 'DELETE' })
  },
}
