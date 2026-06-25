import { useCallback, useEffect, useRef, useState } from 'react'
import { orderApi } from '@entities/order'
import { orderStorage } from '@shared/lib/storage'
import { ApiError } from '@shared/api'
import type { OrderDetail } from '@entities/order'

const POLL_INTERVAL_MS =
  typeof import.meta.env.VITE_POLL_INTERVAL_MS === 'string' &&
  import.meta.env.VITE_POLL_INTERVAL_MS !== ''
    ? Number(import.meta.env.VITE_POLL_INTERVAL_MS)
    : 10_000

const FAST_POLL_INTERVAL_MS = 2_000

interface State {
  order: OrderDetail | null
  loading: boolean
  error: string | null
  canManage: boolean
}

export interface PollOrderOptions {
  /** После SuccessURL Robokassa: быстрый опрос и sync-payment, пока оплата не подтверждена. */
  confirmPayment?: boolean
}

export function usePollOrderStatus(orderId: string | null, options?: PollOrderOptions) {
  const confirmPayment = options?.confirmPayment ?? false
  const [state, setState] = useState<State>({
    order: null,
    loading: !!orderId,
    error: null,
    canManage: false,
  })
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const syncStartedRef = useRef(false)
  const orderRef = useRef<OrderDetail | null>(null)
  orderRef.current = state.order

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

  const startPolling = useCallback((id: string) => {
    stopPolling()
    const tick = () => { void fetchOnce(id) }
    const fast =
      confirmPayment &&
      orderRef.current?.payment_status === 'pending' &&
      orderRef.current?.generation_status !== 'completed' &&
      orderRef.current?.generation_status !== 'failed'
    timerRef.current = setInterval(tick, fast ? FAST_POLL_INTERVAL_MS : POLL_INTERVAL_MS)
  }, [confirmPayment, fetchOnce, stopPolling])

  useEffect(() => {
    if (!orderId) {
      syncStartedRef.current = false
      setState({ order: null, loading: false, error: null, canManage: false })
      return
    }

    void (async () => {
      if (confirmPayment && !syncStartedRef.current) {
        const token = orderStorage.getAccessToken()
        if (token) {
          syncStartedRef.current = true
          try {
            await orderApi.syncPayment(orderId, token)
          } catch {
            // Вебхук мог уже отработать — продолжаем опрос.
          }
        }
      }
      await fetchOnce(orderId)
      startPolling(orderId)
    })()

    return stopPolling
  }, [orderId, confirmPayment, fetchOnce, startPolling, stopPolling])

  // Переключаем интервал с быстрого на обычный после подтверждения оплаты.
  useEffect(() => {
    if (!orderId || !state.order) return
    if (state.order.payment_status !== 'pending') {
      startPolling(orderId)
    }
  }, [orderId, state.order?.payment_status, startPolling])

  return state
}
