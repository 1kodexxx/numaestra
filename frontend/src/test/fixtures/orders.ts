import type { CreateOrderResponse, OrderDetail, Track } from '@entities/order'

export const ORDER_ID = '11111111-1111-4111-8111-111111111111'
export const ACCESS_TOKEN = 'test-access-token'

export function makeTracks(count = 4): Track[] {
  return Array.from({ length: count }, (_, i) => ({
    index: i + 1,
    audio_url: `https://cdn.example.com/track-${i + 1}.mp3`,
    duration_sec: 180 + i,
  }))
}

export function completedOrder(tracks: Track[] = makeTracks()): OrderDetail {
  return {
    id: ORDER_ID,
    invoice_id: 4242,
    payment_status: 'paid',
    generation_status: 'completed',
    amount_kopecks: 200_000,
    tracks,
    paid_at: new Date().toISOString(),
  }
}

export function pendingOrder(): OrderDetail {
  return {
    id: ORDER_ID,
    invoice_id: 4242,
    payment_status: 'pending',
    generation_status: 'new',
    amount_kopecks: 200_000,
    tracks: [],
  }
}

export function processingOrder(): OrderDetail {
  return {
    id: ORDER_ID,
    invoice_id: 4242,
    payment_status: 'paid',
    generation_status: 'processing',
    amount_kopecks: 200_000,
    tracks: [],
    paid_at: new Date().toISOString(),
  }
}

export function queuedOrder(): OrderDetail {
  return {
    ...processingOrder(),
    generation_status: 'queued',
  }
}

export function createOrderResponse(): CreateOrderResponse {
  return {
    id: ORDER_ID,
    invoice_id: 4242,
    payment_status: 'pending',
    generation_status: 'new',
    amount_kopecks: 200_000,
    payment_url: 'https://auth.robokassa.ru/pay/test-invoice',
    access_token: ACCESS_TOKEN,
  }
}
