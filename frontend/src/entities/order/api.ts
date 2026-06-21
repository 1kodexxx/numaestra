import { apiFetch } from '@shared/api'
import type { CreateOrderResponse, OrderDetail } from './types'

export interface CreateOrderPayload {
  email: string
  phone: string
  brief: string
  plan: string
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
}
