import { apiFetch } from '@shared/api'
import type { CreateOrderResponse, OrderDetail, OrderSummary } from './types'

export interface CreateOrderPayload {
  email: string
  phone: string
  brief: string
  category_id: string
  answers: Record<string, string>
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

  // Список заказов владельца (по email/phone заказа, чей токен передан).
  list(accessToken?: string) {
    return apiFetch<OrderSummary[]>('/orders/', { accessToken })
  },
}
