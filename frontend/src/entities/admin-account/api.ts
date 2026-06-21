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
}
