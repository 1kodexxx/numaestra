import { apiFetch } from '@shared/api'

export interface DashboardStats {
  orders: {
    total: number
    paid: number
    revenue_kopecks: number
    completed: number
    processing: number
    failed: number
    today: number
  }
  accounts: {
    total: number
    active: number
    token_balance: number
  }
  categories_total: number
  examples_total: number
  examples_active: number
}

export const adminStatsApi = {
  get() {
    return apiFetch<DashboardStats>('/admin/stats')
  },
}
