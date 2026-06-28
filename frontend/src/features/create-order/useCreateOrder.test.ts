import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useCreateOrder } from './useCreateOrder'
import { orderApi } from '@entities/order'
import { orderStorage } from '@shared/lib/storage'
import type { CreateOrderResponse } from '@entities/order'

// orderApi.create дёргает реальный fetch — мокаем сам метод, чтобы тест проверял
// поведение хука (сохранение токена + редирект), а не сетевой слой.
vi.mock('@entities/order', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@entities/order')>()
  return { ...actual, orderApi: { ...actual.orderApi, create: vi.fn() } }
})

const payload = {
  email: 'user@example.com',
  phone: '+70000000000',
  brief: 'Песня жене на юбилей',
  category_id: '',
  answers: {},
  consent_doc_version: '2026-06-28',
}

const response: CreateOrderResponse = {
  id: 'order-1',
  invoice_id: 1001,
  payment_status: 'pending',
  generation_status: 'new',
  amount_kopecks: 200000,
  payment_url: 'https://auth.robokassa.ru/pay/order-1',
  access_token: 'tok-xyz',
}

describe('useCreateOrder', () => {
  let hrefSpy: ReturnType<typeof vi.fn>

  beforeEach(() => {
    localStorage.clear()
    vi.mocked(orderApi.create).mockReset()
    // window.location.href в jsdom не присвоить напрямую — подменяем объект.
    hrefSpy = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { set href(v: string) { hrefSpy(v) } },
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('при успехе сохраняет заказ, ведёт на страницу статуса (демо-first) и возвращает id', async () => {
    vi.mocked(orderApi.create).mockResolvedValueOnce(response)

    const { result } = renderHook(() => useCreateOrder())

    let returned: string | null = null
    await act(async () => {
      returned = await result.current.submit(payload)
    })

    expect(returned).toBe('order-1')
    expect(orderStorage.getOrderId()).toBe('order-1')
    expect(orderStorage.getAccessToken()).toBe('tok-xyz')
    // Демо-first: ведём на /status, а не сразу на Robokassa — там клиент слушает
    // демо и оттуда переходит к оплате.
    expect(hrefSpy).toHaveBeenCalledWith('/status/order-1')
    expect(result.current.error).toBeNull()
    // loading намеренно остаётся true: дальше следует редирект, гасить спиннер
    // до ухода со страницы незачем.
    expect(result.current.loading).toBe(true)
  })

  it('выставляет loading=true на время запроса', async () => {
    let resolve!: (r: CreateOrderResponse) => void
    vi.mocked(orderApi.create).mockReturnValueOnce(
      new Promise<CreateOrderResponse>((r) => { resolve = r }),
    )

    const { result } = renderHook(() => useCreateOrder())

    expect(result.current.loading).toBe(false)
    act(() => { void result.current.submit(payload) })

    await waitFor(() => expect(result.current.loading).toBe(true))

    await act(async () => { resolve(response) })
  })

  it('при ошибке сохраняет сообщение, не редиректит и возвращает null', async () => {
    vi.mocked(orderApi.create).mockRejectedValueOnce(new Error('email обязателен'))

    const { result } = renderHook(() => useCreateOrder())

    let returned: string | null = 'sentinel'
    await act(async () => {
      returned = await result.current.submit(payload)
    })

    expect(returned).toBeNull()
    expect(result.current.error).toBe('email обязателен')
    expect(result.current.loading).toBe(false)
    expect(hrefSpy).not.toHaveBeenCalled()
    expect(orderStorage.getOrderId()).toBeNull()
  })
})
