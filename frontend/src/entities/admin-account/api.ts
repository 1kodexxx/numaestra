import { apiFetch } from '@shared/api'
import type { AccountStatus, AddAccountPayload, AdminAccount } from './types'

export const adminAccountApi = {
  list() {
    return apiFetch<AdminAccount[]>('/admin/accounts/')
  },

  add(payload: AddAccountPayload) {
    return apiFetch<AdminAccount>('/admin/accounts/', { method: 'POST', body: payload })
  },

  setStatus(id: string, status: AccountStatus) {
    return apiFetch<void>(`/admin/accounts/${id}`, { method: 'PATCH', body: { status } })
  },

  // reset полностью «достаёт» зависший аккаунт: статус active, сброс счётчика
  // ошибок и паузы, освобождение занятых слотов.
  reset(id: string) {
    return apiFetch<void>(`/admin/accounts/${id}/reset`, { method: 'POST' })
  },
}
