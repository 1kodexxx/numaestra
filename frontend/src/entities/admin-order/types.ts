export interface AdminTrack {
  index: number
  audio_url: string
}

export interface AdminOrder {
  id: string
  invoice_id: number
  email: string
  phone: string
  brief: string
  amount_kopecks: number
  payment_status: 'pending' | 'paid' | 'failed' | 'refunded'
  generation_status: 'new' | 'queued' | 'processing' | 'completed' | 'failed'
  tracks: AdminTrack[]
  admin_feedback?: string
  admin_feedback_at?: string
  consent_given_at?: string
  consent_doc_version?: string
  created_at: string
  // Демо-ассеты: превью с водяным знаком (demo_url) и полные клипы (demo_clips).
  demo_status?: 'none' | 'processing' | 'ready' | 'failed'
  demo_url?: string
  demo_clips?: AdminTrack[]
}

export interface AdminOrderListResponse {
  orders: AdminOrder[]
  total: number
}
