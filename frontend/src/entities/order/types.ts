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
  original_amount_kopecks?: number
  discount_kopecks?: number
  payment_url: string
  access_token: string
}

export type DemoStatus = 'none' | 'processing' | 'ready' | 'failed' | 'limited'

// Ответ GET /api/v1/orders/:id (детали)
export interface OrderDetail {
  id: string
  invoice_id: number
  payment_status: PaymentStatus
  generation_status: GenerationStatus
  amount_kopecks: number
  tracks: Track[]
  paid_at?: string
  generation_phase?: string
  generation_progress?: number
  tracks_ready?: number
  share_revoked?: boolean
  // Демо-фрагмент (бесплатный, до оплаты). demo_url заполнен только при ready.
  demo_status?: DemoStatus
  demo_url?: string
  // Момент последнего изменения (RFC3339). Пока pending+demo processing — это старт
  // демо: серверный якорь прогресса демо, переживающий перезагрузку страницы.
  updated_at?: string
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
