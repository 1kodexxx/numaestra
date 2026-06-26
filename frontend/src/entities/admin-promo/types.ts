export interface PromoCode {
  id: string
  code: string
  discount_type: 'percent' | 'fixed_rub'
  discount_value: number
  max_uses: number | null
  current_uses: number
  valid_from?: string
  valid_until?: string
  is_active: boolean
  description: string
  created_at: string
}

export interface CreatePromoPayload {
  code: string
  discount_type: 'percent' | 'fixed_rub'
  discount_value: number
  max_uses?: number
  valid_from?: string
  valid_until?: string
  description?: string
}

export interface UpdatePromoPayload {
  description: string
  is_active: boolean
  max_uses?: number
  valid_from?: string
  valid_until?: string
}
