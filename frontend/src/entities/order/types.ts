export type PaymentStatus = 'pending' | 'paid'

export type GenerationStatus = 'new' | 'queued' | 'processing' | 'completed' | 'failed'

export interface Track {
  index: number
  audio_url: string
  duration_sec: number
}

// Ответ POST /api/v1/orders (создание)
export interface CreateOrderResponse {
  id: string
  invoice_id: number
  payment_status: PaymentStatus
  generation_status: GenerationStatus
  amount_kopecks: number
  payment_url: string
  access_token: string
}

// Ответ GET /api/v1/orders/:id (детали)
export interface OrderDetail {
  id: string
  invoice_id: number
  payment_status: PaymentStatus
  generation_status: GenerationStatus
  amount_kopecks: number
  tracks: Track[]
  paid_at?: string // RFC3339; якорь для прогресс-бара генерации
}

// Элемент ответа GET /api/v1/orders/ (список заказов владельца по токену)
export interface OrderSummary {
  id: string
  invoice_id: number
  brief: string
  payment_status: string
  generation_status: string
  tracks_count: number
}
