import { apiFetch } from '@shared/api'
import type { Category, WizardData } from './types'

export const categoryApi = {
  list() {
    return apiFetch<Category[]>('/categories/')
  },

  wizard(categoryId: string) {
    return apiFetch<WizardData>(`/categories/${categoryId}/wizard`)
  },
}
