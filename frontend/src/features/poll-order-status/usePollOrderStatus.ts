import { useCallback, useEffect, useRef, useState } from 'react'
import { orderApi } from '@entities/order'
import { orderStorage } from '@shared/lib/storage'
import { ApiError } from '@shared/api'
import type { OrderDetail } from '@entities/order'

// В E2E-сборке можно ускорить через VITE_POLL_INTERVAL_MS (см. npm run build:e2e).
const POLL_INTERVAL_MS =
  typeof import.meta.env.VITE_POLL_INTERVAL_MS === 'string' &&
  import.meta.env.VITE_POLL_INTERVAL_MS !== ''
    ? Number(import.meta.env.VITE_POLL_INTERVAL_MS)
    : 10_000

interface State {
  order: OrderDetail | null
  loading: boolean
  error: string | null
  /** Есть валидный access_token — оплата, список заказов, отзыв share. */
  canManage: boolean
}

export function usePollOrderStatus(orderId: string | null) {
  const [state, setState] = useState<State>({
    order: null,
    loading: !!orderId,
    error: null,
    canManage: false,
  })
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const stopPolling = useCallback(() => {
    if (timerRef.current) {
      clearInterval(timerRef.current)
      timerRef.current = null
    }
  }, [])

  const fetchOnce = useCallback(async (id: string) => {
    const token = orderStorage.getAccessToken() ?? undefined

    if (token) {
      try {
        const order = await orderApi.getById(id, token)
        setState({ order, loading: false, error: null, canManage: true })
        const terminal = order.generation_status === 'completed' || order.generation_status === 'failed'
        if (terminal) stopPolling()
        return
      } catch (err: unknown) {
        if (err instanceof ApiError && err.status === 401) {
          // Токен от удалённого или устаревшего заказа — сбрасываем и пробуем публичный статус.
          orderStorage.clear()
        } else {
          const message = err instanceof Error ? err.message : 'Ошибка загрузки заказа'
          setState({ order: null, loading: false, error: message, canManage: false })
          stopPolling()
          return
        }
      }
    }

    try {
      const order = await orderApi.getPublicStatus(id)
      setState({ order, loading: false, error: null, canManage: false })
      const terminal = order.generation_status === 'completed' || order.generation_status === 'failed'
      if (terminal) stopPolling()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Ошибка загрузки заказа'
      setState({ order: null, loading: false, error: message, canManage: false })
      stopPolling()
    }
  }, [stopPolling])

  useEffect(() => {
    if (!orderId) {
      setState({ order: null, loading: false, error: null, canManage: false })
      return
    }

    fetchOnce(orderId)
    timerRef.current = setInterval(() => fetchOnce(orderId), POLL_INTERVAL_MS)

    return stopPolling
  }, [orderId, fetchOnce, stopPolling])

  return state
}
