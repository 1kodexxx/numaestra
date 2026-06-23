import { renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { usePollOrderStatus } from './usePollOrderStatus'
import { orderApi } from '@entities/order'
import { orderStorage } from '@shared/lib/storage'
import type { OrderDetail } from '@entities/order'

vi.mock('@entities/order', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@entities/order')>()
  return { ...actual, orderApi: { ...actual.orderApi, getById: vi.fn() } }
})

function order(status: OrderDetail['generation_status']): OrderDetail {
  return {
    id: 'order-1',
    invoice_id: 1001,
    payment_status: 'paid',
    generation_status: status,
    amount_kopecks: 200000,
    tracks: [],
  }
}

describe('usePollOrderStatus', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
    vi.mocked(orderApi.getById).mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('ничего не грузит при orderId=null', () => {
    const { result } = renderHook(() => usePollOrderStatus(null))

    expect(orderApi.getById).not.toHaveBeenCalled()
    expect(result.current.loading).toBe(false)
    expect(result.current.order).toBeNull()
  })

  it('сразу делает первый запрос и передаёт сохранённый токен', async () => {
    orderStorage.saveOrder('order-1', 'tok-abc')
    vi.mocked(orderApi.getById).mockResolvedValue(order('processing'))

    const { result } = renderHook(() => usePollOrderStatus('order-1'))

    await vi.waitFor(() => expect(result.current.order?.generation_status).toBe('processing'))
    expect(orderApi.getById).toHaveBeenCalledWith('order-1', 'tok-abc')
    expect(result.current.loading).toBe(false)
  })

  it('повторно опрашивает каждые 10 секунд, пока статус не терминальный', async () => {
    vi.mocked(orderApi.getById)
      .mockResolvedValueOnce(order('queued'))
      .mockResolvedValueOnce(order('processing'))
      .mockResolvedValue(order('completed'))

    renderHook(() => usePollOrderStatus('order-1'))

    // первый вызов — сразу при монтировании
    await vi.waitFor(() => expect(orderApi.getById).toHaveBeenCalledTimes(1))

    await vi.advanceTimersByTimeAsync(10_000)
    expect(orderApi.getById).toHaveBeenCalledTimes(2)

    await vi.advanceTimersByTimeAsync(10_000)
    expect(orderApi.getById).toHaveBeenCalledTimes(3)
  })

  it('останавливает поллинг при статусе completed', async () => {
    vi.mocked(orderApi.getById).mockResolvedValue(order('completed'))

    const { result } = renderHook(() => usePollOrderStatus('order-1'))

    await vi.waitFor(() => expect(result.current.order?.generation_status).toBe('completed'))

    await vi.advanceTimersByTimeAsync(30_000)
    // после терминального статуса новых запросов нет
    expect(orderApi.getById).toHaveBeenCalledTimes(1)
  })

  it('останавливает поллинг при статусе failed', async () => {
    vi.mocked(orderApi.getById).mockResolvedValue(order('failed'))

    renderHook(() => usePollOrderStatus('order-1'))

    await vi.waitFor(() => expect(orderApi.getById).toHaveBeenCalledTimes(1))
    await vi.advanceTimersByTimeAsync(30_000)
    expect(orderApi.getById).toHaveBeenCalledTimes(1)
  })

  it('сохраняет ошибку и прекращает поллинг при сбое запроса', async () => {
    vi.mocked(orderApi.getById).mockRejectedValue(new Error('заказ не найден'))

    const { result } = renderHook(() => usePollOrderStatus('order-1'))

    await vi.waitFor(() => expect(result.current.error).toBe('заказ не найден'))
    expect(result.current.order).toBeNull()

    await vi.advanceTimersByTimeAsync(30_000)
    expect(orderApi.getById).toHaveBeenCalledTimes(1)
  })
})
