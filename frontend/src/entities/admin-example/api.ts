import { apiFetch, apiUpload } from '@shared/api'
import type { AdminExample, ExamplePayload } from './types'

export const adminExampleApi = {
  list() {
    return apiFetch<AdminExample[]>('/admin/examples/')
  },

  create(payload: ExamplePayload) {
    return apiFetch<AdminExample>('/admin/examples/', { method: 'POST', body: payload })
  },

  update(id: string, payload: Omit<ExamplePayload, 'id'>) {
    return apiFetch<AdminExample>(`/admin/examples/${id}`, { method: 'PUT', body: payload })
  },

  remove(id: string) {
    return apiFetch<void>(`/admin/examples/${id}`, { method: 'DELETE' })
  },

  uploadCover(id: string, file: File) {
    return apiUpload<{ cover_url: string }>(`/admin/examples/${id}/cover`, file)
  },
}
