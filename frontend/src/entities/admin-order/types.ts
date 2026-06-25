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
}

export interface AdminOrderListResponse {
  orders: AdminOrder[]
  total: number
}
