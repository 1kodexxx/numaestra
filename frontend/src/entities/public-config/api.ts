import { apiFetch } from '@shared/api'

export interface PublicConfig {
  price_kopecks: number
  price_label: string
  /** Зачёркнутая «старая» цена (маркетинг); отсутствует, если показывать нечего. */
  old_price_label?: string
  /** Доступно ли демо; false → воронка «оплата сразу» одним платежом. */
  demo_enabled: boolean
  /** Цена демо в копейках. 0 → демо бесплатное. */
  demo_price_kopecks: number
  /** Метка цены демо, например «50 ₽». Пусто → демо бесплатное. */
  demo_price_label?: string
  /** Сколько остаётся доплатить после демо, например «940 ₽». */
  remaining_price_label?: string
  consent_doc_version: string
}

export const publicConfigApi = {
  get() {
    return apiFetch<PublicConfig>('/public/config')
  },
}
