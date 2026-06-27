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

    // handleFetchError решает, останавливать ли опрос. Терминально только когда
    // заказа точно нет (404); транзиентные ошибки (сеть, 5xx, 429) НЕ
    // останавливают опрос — следующий тик повторит. Если заказ уже загружен —
    // тихо игнорируем сбой; иначе показываем мягкую ошибку, но продолжаем попытки.
    const handleFetchError = (err: unknown): boolean => {
      const status = err instanceof ApiError ? err.status : undefined
      if (status === 404) {
        setState(s => ({ ...s, order: null, loading: false, error: 'Заказ не найден', canManage: false }))
        stopPolling()
        clearStaleTimer()
        return false
      }
      if (!orderRef.current) {
        const message = err instanceof Error ? err.message : 'Ошибка загрузки заказа'
        setState(s => ({ ...s, loading: false, error: message }))
      }
      return true // транзиентная ошибка — продолжаем опрос
    }

    // tryPublic — публичный статус без токена. Фоллбэк, когда токена нет либо он
    // не подошёл (401/403) или приватный запрос временно упал.
    const tryPublic = async (): Promise<boolean> => {
      try {
        const order = await orderApi.getPublicStatus(id)
        return applyOrder(order, false)
      } catch (err: unknown) {
        return handleFetchError(err)
      }
    }

    if (token) {
      try {
        const order = await orderApi.getById(id, token)
        return applyOrder(order, true)
      } catch (err: unknown) {
        const status = err instanceof ApiError ? err.status : undefined
        // 401 — токен невалиден: чистим его и продолжаем по публичному статусу.
        if (status === 401) orderStorage.clear()
        // 403 (токен не для этого заказа), 5xx, сеть — пробуем публичный статус,
        // а не глушим опрос: заказ мог быть открыт по ссылке без прав управления.
        return tryPublic()
      }
    }

    return tryPublic()
  }, [stopPolling, startStaleTimer, clearStaleTimer])

  const startPolling = useCallback((id: string) => {
    stopPolling()
    const tick = () => { void fetchOnce(id) }
    const o = orderRef.current
    const notTerminal = o?.generation_status !== 'completed' && o?.generation_status !== 'failed'
    // Быстрый опрос: после возврата с оплаты И пока готовится бесплатное демо
    // (чтобы плеер появился сразу по готовности — «процесс приготовления» вживую).
    const fast =
      notTerminal &&
      ((confirmPayment && o?.payment_status === 'pending') ||
        o?.demo_status === 'processing')
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
      // sync-payment вызываем без токена (эндпоинт публичный) — исправляет
      // сценарий «другой браузер / очищенный localStorage» когда webhook не дошёл.
      if (confirmPayment && !syncStartedRef.current) {
        syncStartedRef.current = true
        const token = orderStorage.getAccessToken() ?? undefined
        try {
          await orderApi.syncPayment(orderId, token)
        } catch {
          // Вебхук мог уже отработать — продолжаем опрос.
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

  // Когда демо перестало готовиться (ready/failed), пересобираем интервал —
  // возвращаемся с быстрого опроса на обычный.
  useEffect(() => {
    if (!orderId || !state.order) return
    if (state.order.generation_status === 'completed' || state.order.generation_status === 'failed') return
    startPolling(orderId)
  }, [orderId, state.order?.demo_status, state.order?.generation_status, startPolling])

  // Пока оплата не подтверждена — повторяем sync-payment на каждом быстром тике.
  // Это покрывает сценарий: первый sync вернул { synced: false } (OpState не готов),
  // webhook не дошёл, пользователь ждёт на странице.
  const syncRetryCountRef = useRef(0)
  useEffect(() => {
    if (!confirmPayment || !orderId) return
    if (state.order && state.order.payment_status !== 'pending') return
    if (syncRetryCountRef.current >= 10) return   // max 10 повторов ≈ 20 с
    const t = setTimeout(async () => {
      syncRetryCountRef.current += 1
      try {
        const token = orderStorage.getAccessToken() ?? undefined
        await orderApi.syncPayment(orderId, token)
      } catch { /* ignore */ }
    }, FAST_POLL_INTERVAL_MS)
    return () => clearTimeout(t)
  }, [confirmPayment, orderId, state.order?.payment_status])

  return state
}
