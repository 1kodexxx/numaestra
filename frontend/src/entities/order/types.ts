export type PaymentStatus = 'pending' | 'paid'

export type GenerationStatus = 'new' | 'queued' | 'processing' | 'completed' | 'failed'

export interface Track {
  index: number
  audio_url: string
  duration_sec: number
}

// Статус оплаты демо — вторая платёжная полоса заказа. Пусто/undefined означает,
// что у заказа нет отдельного счёта на демо (легаси или бесплатный заказ).
export type DemoPaymentStatus = 'pending' | 'paid' | 'failed' | 'refunded'

// Поля платного демо, общие для ответов создания заказа и его статуса.
export interface DemoPaymentFields {
  /** Сколько осталось доплатить за песню с учётом зачёта демо. */
  remaining_kopecks?: number
  /** Отдельный InvId Robokassa для счёта за демо. */
  demo_invoice_id?: number
  /** Цена демо; отсутствует, если демо-счёта у заказа нет. */
  demo_amount_kopecks?: number
  demo_payment_status?: DemoPaymentStatus
  /** Ссылка на оплату демо; пусто, если демо уже оплачено или счёта нет. */
  demo_payment_url?: string
}

// Ответ POST /api/v1/orders (создание)
export interface CreateOrderResponse extends DemoPaymentFields {
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
export interface OrderDetail extends DemoPaymentFields {
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
  // Демо-фрагмент (до оплаты песни). demo_url заполнен только при ready.
  demo_status?: DemoStatus
  demo_url?: string
  /** Ссылка на доплату за песню; пусто для уже оплаченного заказа. */
  payment_url?: string
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
