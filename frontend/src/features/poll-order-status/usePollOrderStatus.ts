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

/** После этого времени (мс) без терминального статуса показываем предупреждение. */
const STALE_TIMEOUT_MS = 8 * 60 * 1000

interface State {
  order: OrderDetail | null
  loading: boolean
  error: string | null
  canManage: boolean
  pollingTooLong: boolean
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
    pollingTooLong: false,
  })
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const staleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const syncStartedRef = useRef(false)
  const orderRef = useRef<OrderDetail | null>(null)
  orderRef.current = state.order

  const clearStaleTimer = useCallback(() => {
    if (staleTimerRef.current) {
      clearTimeout(staleTimerRef.current)
      staleTimerRef.current = null
    }
  }, [])

  const startStaleTimer = useCallback(() => {
    clearStaleTimer()
    staleTimerRef.current = setTimeout(() => {
      setState(s => ({ ...s, pollingTooLong: true }))
    }, STALE_TIMEOUT_MS)
  }, [clearStaleTimer])

  const stopPolling = useCallback(() => {
    if (timerRef.current) {
      clearInterval(timerRef.current)
      timerRef.current = null
    }
  }, [])

  const fetchOnce = useCallback(async (id: string): Promise<boolean> => {
    const token = orderStorage.getAccessToken() ?? undefined

    const applyOrder = (order: OrderDetail, canManage: boolean) => {
      const terminal = order.generation_status === 'completed' || order.generation_status === 'failed'
      const activeGeneration = order.payment_status !== 'pending' && !terminal
      setState(s => ({
        order,
        loading: false,
        error: null,
        canManage,
        pollingTooLong: terminal ? false : s.pollingTooLong,
      }))
      if (terminal) {
        stopPolling()
        clearStaleTimer()
      } else if (activeGeneration && !staleTimerRef.current) {
        startStaleTimer()
      }
      return !terminal
    }

    if (token) {
      try {
        const order = await orderApi.getById(id, token)
        return applyOrder(order, true)
      } catch (err: unknown) {
        if (err instanceof ApiError && err.status === 401) {
          orderStorage.clear()
        } else {
          const message = err instanceof Error ? err.message : 'Ошибка загрузки заказа'
          setState(s => ({ ...s, order: null, loading: false, error: message, canManage: false }))
          stopPolling()
          clearStaleTimer()
          return false
        }
      }
    }

    try {
      const order = await orderApi.getPublicStatus(id)
      return applyOrder(order, false)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Ошибка загрузки заказа'
      setState(s => ({ ...s, order: null, loading: false, error: message, canManage: false }))
      stopPolling()
      clearStaleTimer()
      return false
    }
  }, [stopPolling, startStaleTimer, clearStaleTimer])

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
      clearStaleTimer()
      setState({ order: null, loading: false, error: null, canManage: false, pollingTooLong: false })
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
      const keepPolling = await fetchOnce(orderId)
      if (keepPolling) {
        startPolling(orderId)
      }
    })()

    return () => { stopPolling(); clearStaleTimer() }
  }, [orderId, confirmPayment, fetchOnce, startPolling, stopPolling, clearStaleTimer])

  // После подтверждения оплаты переключаем интервал с 2 с на 10 с.
  useEffect(() => {
    if (!confirmPayment || !orderId || !state.order) return
    const terminal =
      state.order.generation_status === 'completed' || state.order.generation_status === 'failed'
    if (terminal || state.order.payment_status === 'pending') return
    startPolling(orderId)
  }, [confirmPayment, orderId, state.order?.payment_status, state.order?.generation_status, startPolling])

  return state
}
