import { apiFetch } from '@shared/api'
import type { AdminMe } from './types'

export const adminAuthApi = {
  login(login: string, password: string) {
    return apiFetch<AdminMe>('/admin/login', { method: 'POST', body: { login, password } })
  },

  logout() {
    return apiFetch<void>('/admin/logout', { method: 'POST' })
  },

  me() {
    return apiFetch<AdminMe>('/admin/me')
  },
}
