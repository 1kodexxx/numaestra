import { apiFetch } from '@shared/api'
import type { CreateOrderResponse, OrderDetail, OrderSummary, Track } from './types'

export interface CreateOrderPayload {
  email: string
  phone: string
  brief: string
  category_id: string
  answers: Record<string, string>
  consent_doc_version: string
}

export const orderApi = {
  create(payload: CreateOrderPayload) {
    return apiFetch<CreateOrderResponse>('/orders/', {
      method: 'POST',
      body: payload,
    })
  },

  getById(id: string, accessToken?: string) {
    return apiFetch<OrderDetail>(`/orders/${id}`, { accessToken })
  },

  // Заново получить ссылку на оплату для неоплаченного заказа (повторная оплата).
  paymentUrl(id: string, accessToken?: string) {
    return apiFetch<{ payment_url: string }>(`/orders/${id}/payment-url`, { accessToken })
  },

  // Публичная карточка завершённой песни для шеринга — без токена.
  getPublicShare(id: string) {
    return apiFetch<{ id: string; tracks: Track[] }>(`/orders/${id}/share`)
  },

  // Список заказов владельца (по email/phone заказа, чей токен передан).
  list(accessToken?: string) {
    return apiFetch<OrderSummary[]>('/orders/', { accessToken })
  },
}
