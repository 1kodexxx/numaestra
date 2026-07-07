import { apiFetch } from '@shared/api'

export interface PublicConfig {
  price_kopecks: number
  price_label: string
  /** Зачёркнутая «старая» цена (маркетинг); отсутствует, если показывать нечего. */
  old_price_label?: string
  consent_doc_version: string
}

export const publicConfigApi = {
  get() {
    return apiFetch<PublicConfig>('/public/config')
  },
}
