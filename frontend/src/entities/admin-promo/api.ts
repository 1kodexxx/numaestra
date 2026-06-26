import { apiFetch } from '@shared/api'
import type { CreatePromoPayload, PromoCode, UpdatePromoPayload } from './types'

export const adminPromoApi = {
  list(page = 0, limit = 50) {
    return apiFetch<{ items: PromoCode[]; total: number }>(
      `/admin/promo-codes?limit=${limit}&offset=${page * limit}`,
    )
  },

  create(payload: CreatePromoPayload) {
    return apiFetch<PromoCode>('/admin/promo-codes', { method: 'POST', body: payload })
  },

  update(id: string, payload: UpdatePromoPayload) {
    return apiFetch<PromoCode>(`/admin/promo-codes/${id}`, { method: 'PUT', body: payload })
  },

  delete(id: string) {
    return apiFetch<{ deleted: boolean }>(`/admin/promo-codes/${id}`, { method: 'DELETE' })
  },
}

export const promoApi = {
  validate(code: string, amountKopecks?: number) {
    const q = amountKopecks ? `?amount_kopecks=${amountKopecks}` : ''
    return apiFetch<{
      code: string
      discount_type: string
      discount_value: number
      discount_kopecks: number
    }>(`/promo/${encodeURIComponent(code)}${q}`)
  },
}
