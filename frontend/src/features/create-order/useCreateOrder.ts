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
    if (inFlightRef.current) return null
    inFlightRef.current = true
    setState({ loading: true, error: null })
    try {
      const result = await orderApi.create(payload)
      orderStorage.saveOrder(result.id, result.access_token)
      reachGoal(GOALS.ORDER_SUBMIT, { category_id: payload.category_id || 'custom' })
      // Перенаправляем на страницу оплаты Robokassa
      window.location.href = result.payment_url
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
