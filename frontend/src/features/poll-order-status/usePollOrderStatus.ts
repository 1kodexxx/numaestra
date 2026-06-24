import { useCallback, useEffect, useRef, useState } from 'react'
import { orderApi } from '@entities/order'
import { orderStorage } from '@shared/lib/storage'
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
}

export function usePollOrderStatus(orderId: string | null) {
  const [state, setState] = useState<State>({ order: null, loading: !!orderId, error: null })
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const stopPolling = useCallback(() => {
    if (timerRef.current) {
      clearInterval(timerRef.current)
      timerRef.current = null
    }
  }, [])

  const fetchOnce = useCallback(async (id: string) => {
    const token = orderStorage.getAccessToken() ?? undefined
    try {
      const order = await orderApi.getById(id, token)
      setState({ order, loading: false, error: null })
      const terminal = order.generation_status === 'completed' || order.generation_status === 'failed'
      if (terminal) stopPolling()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Ошибка загрузки заказа'
      setState({ order: null, loading: false, error: message })
      stopPolling()
    }
  }, [stopPolling])

  useEffect(() => {
    if (!orderId) {
      setState({ order: null, loading: false, error: null })
      return
    }

    fetchOnce(orderId)
    timerRef.current = setInterval(() => fetchOnce(orderId), POLL_INTERVAL_MS)

    return stopPolling
  }, [orderId, fetchOnce, stopPolling])

  return state
}
