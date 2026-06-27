import { useRef, useState } from 'react'
import { orderApi } from '@entities/order'
import { orderStorage } from '@shared/lib/storage'
import { GOALS, reachGoal } from '@shared/lib/analytics'
import type { CreateOrderPayload } from '@entities/order'

interface State {
  loading: boolean
  error: string | null
}

export function useCreateOrder() {
  const [state, setState] = useState<State>({ loading: false, error: null })
  const inFlightRef = useRef(false)

  async function submit(payload: CreateOrderPayload): Promise<string | null> {
    if (inFlightRef.current) {
      // Не молчим: даём понять, что заказ уже создаётся (двойной клик/таб).
      setState((s) => ({ ...s, error: 'Заказ уже создаётся — подождите несколько секунд.' }))
      return null
    }
    inFlightRef.current = true
    setState({ loading: true, error: null })
    try {
      // Стабильный ключ на сессию: при сетевом таймауте повторный submit
      // с тем же UUID не создаёт дубль заказа (idempotency middleware на бэкенде).
      const idempotencyKey = orderStorage.getOrCreateIdempotencyKey()
      const result = await orderApi.create(payload, idempotencyKey)
      orderStorage.saveOrder(result.id, result.access_token)
      orderStorage.saveInvoiceOrder(result.invoice_id, result.id)
      reachGoal(GOALS.ORDER_SUBMIT, { category_id: payload.category_id || 'custom' })
      // Ключ идемпотентности чистим при ФАКТИЧЕСКОМ уходе со страницы (pagehide),
      // а не сразу: если редирект почему-то не случится (заблокирован, ошибка),
      // ключ переживёт, и повторная отправка не создаст дубль заказа.
      window.addEventListener('pagehide', () => orderStorage.clearIdempotencyKey(), { once: true })
      // Демо-first воронка: всегда ведём на страницу статуса. Там клиент видит
      // «приготовление» бесплатного демо, слушает фрагмент и оттуда переходит к
      // оплате (кнопка «Перейти к оплате» запрашивает свежий payment-url).
      // Прямой редирект на Robokassa лишал бы клиента демо до оплаты — это и был
      // недоделанный демо-UX. Бесплатный заказ (100% промокод) тоже идёт на статус.
      window.location.href = `/status/${result.id}`
      return result.id
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Неизвестная ошибка'
      setState({ loading: false, error: message })
      inFlightRef.current = false
      return null
    }
  }

  return { ...state, submit }
}
