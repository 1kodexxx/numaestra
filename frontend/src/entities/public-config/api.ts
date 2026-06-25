import { apiFetch } from '@shared/api'

export interface PublicConfig {
  price_kopecks: number
  price_label: string
  consent_doc_version: string
}

export const publicConfigApi = {
  get() {
    return apiFetch<PublicConfig>('/public/config')
  },
}
