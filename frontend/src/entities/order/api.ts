import { apiFetch } from '@shared/api'
import type { CreateOrderResponse, OrderDetail, OrderSummary, Track } from './types'

export interface CreateOrderPayload {
  email: string
  phone: string
  brief: string
  category_id: string
  answers: Record<string, string>
  consent_doc_version: string
  promo_code?: string
  referral_code?: string
}

export const orderApi = {
  create(payload: CreateOrderPayload, idempotencyKey?: string) {
    return apiFetch<CreateOrderResponse>('/orders/', {
      method: 'POST',
      body: payload,
      // Имя заголовка должно совпадать с тем, что читает idempotencyMiddleware на
      // бэкенде (Idempotency-Key, без префикса X-). Иначе ключ не дойдёт и защита
      // от дублей при двойной отправке/сетевом ретрае молча отключится.
      headers: { 'Idempotency-Key': idempotencyKey ?? crypto.randomUUID() },
    })
  },

  getById(id: string, accessToken?: string) {
    return apiFetch<OrderDetail>(`/orders/${id}`, { accessToken })
  },

  // Заново получить ссылку на оплату для неоплаченного заказа (повторная оплата).
  paymentUrl(id: string, accessToken?: string) {
    return apiFetch<{ payment_url: string }>(`/orders/${id}/payment-url`, { accessToken })
  },

  syncPayment(id: string, accessToken?: string) {
    return apiFetch<{ synced: boolean }>(`/orders/${id}/sync-payment`, {
      method: 'POST',
      accessToken,
    })
  },

  // Применить промокод к уже созданному неоплаченному заказу — пересчитывает сумму.
  applyPromo(id: string, promoCode: string, accessToken?: string) {
    return apiFetch<{ amount_kopecks: number; discount_kopecks: number }>(
      `/orders/${id}/apply-promo`,
      { method: 'POST', body: { promo_code: promoCode }, accessToken },
    )
  },

  getPublicStatus(id: string) {
    return apiFetch<OrderDetail>(`/orders/${id}/status`)
  },

  // Публичная карточка завершённой песни для шеринга — без токена.
  getPublicShare(id: string) {
    return apiFetch<{ id: string; tracks: Track[] }>(`/orders/${id}/share`)
  },

  // Список заказов владельца (по email/phone заказа, чей токен передан).
  list(accessToken?: string) {
    return apiFetch<OrderSummary[]>('/orders/', { accessToken })
  },

  revokeShare(id: string, accessToken?: string) {
    return apiFetch<{ share_revoked: boolean }>(`/orders/${id}/share/revoke`, {
      method: 'POST',
      accessToken,
    })
  },

  restoreShare(id: string, accessToken?: string) {
    return apiFetch<{ share_revoked: boolean }>(`/orders/${id}/share/restore`, {
      method: 'POST',
      accessToken,
    })
  },

  requestAccessLink(id: string, email: string) {
    return apiFetch<{ message: string }>(`/orders/${id}/access-link`, {
      method: 'POST',
      body: { email },
    })
  },

  // Резолв UUID заказа по номеру счёта Robokassa (InvId).
  // Используется PaymentReturnPage на новом устройстве / в другом браузере.
  getOrderIdByInvoice(invoiceId: number) {
    return apiFetch<{ id: string }>(`/orders/by-invoice/${invoiceId}`)
  },
}
