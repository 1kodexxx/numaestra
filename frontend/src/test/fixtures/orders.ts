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

/**
 * Неоплаченный заказ с ВЫСТАВЛЕННЫМ счётом на демо — первый шаг платной воронки.
 * Отличается от pendingOrder наличием demo_amount_kopecks: по нему страница
 * статуса решает, показывать «Послушать демо за N ₽» или сразу «Перейти к оплате».
 */
export function demoUnpaidOrder(): OrderDetail {
  return {
    id: ORDER_ID,
    invoice_id: 4242,
    payment_status: 'pending',
    generation_status: 'new',
    amount_kopecks: 99_000,
    // До оплаты демо к оплате стоит полная сумма: заказ можно закрыть одним платежом.
    remaining_kopecks: 99_000,
    demo_invoice_id: 4243,
    demo_amount_kopecks: 5_000,
    demo_payment_status: 'pending',
    demo_status: 'none',
    tracks: [],
  }
}

/** Демо оплачено и готово — остаётся доплатить остаток за песню. */
export function demoPaidOrder(): OrderDetail {
  return {
    id: ORDER_ID,
    invoice_id: 4242,
    payment_status: 'pending',
    generation_status: 'new',
    amount_kopecks: 99_000,
    remaining_kopecks: 94_000,
    demo_invoice_id: 4243,
    demo_amount_kopecks: 5_000,
    demo_payment_status: 'paid',
    demo_status: 'ready',
    demo_url: 'https://cdn.example.com/demo.mp3',
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
    generation_phase: 'generating',
    generation_progress: 55,
    tracks_ready: 2,
  }
}

export function queuedOrder(): OrderDetail {
  return {
    ...processingOrder(),
    generation_status: 'queued',
    generation_phase: 'queued',
    generation_progress: 3,
    tracks_ready: 0,
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
