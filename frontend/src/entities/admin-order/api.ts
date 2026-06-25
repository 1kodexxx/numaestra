import { apiFetch } from '@shared/api'
import type { AdminOrder, AdminOrderListResponse } from './types'

export const adminOrderApi = {
  list(page: number, perPage: number) {
    return apiFetch<AdminOrderListResponse>(`/admin/orders/?page=${page}&per_page=${perPage}`)
  },

  get(id: string) {
    return apiFetch<AdminOrder>(`/admin/orders/${id}`)
  },

  refund(id: string) {
    return apiFetch<void>(`/admin/orders/${id}/refund`, { method: 'POST' })
  },

  confirmPayment(id: string) {
    return apiFetch<void>(`/admin/orders/${id}/confirm-payment`, { method: 'POST' })
  },

  // Повторно поставить оплаченный, но упавший заказ в очередь генерации.
  regenerate(id: string) {
    return apiFetch<void>(`/admin/orders/${id}/regenerate`, { method: 'POST' })
  },

  sendFeedback(id: string, message: string) {
    return apiFetch<void>(`/admin/orders/${id}/feedback`, { method: 'POST', body: { message } })
  },

  remove(id: string) {
    return apiFetch<void>(`/admin/orders/${id}`, { method: 'DELETE' })
  },
}
