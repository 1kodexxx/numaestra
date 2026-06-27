import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resolveOrderId } from './resolveOrderId'
import { orderApi } from '@entities/order'
import { orderStorage } from '@shared/lib/storage'

vi.mock('@entities/order', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@entities/order')>()
  return { ...actual, orderApi: { ...actual.orderApi, getOrderIdByInvoice: vi.fn() } }
})

describe('resolveOrderId (P0-4: возврат с оплаты не должен вести на чужой заказ)', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.mocked(orderApi.getOrderIdByInvoice).mockReset()
  })
  afterEach(() => vi.clearAllMocks())

  it('InvId есть в локальной карте → возвращает сопоставленный заказ, без API', async () => {
    orderStorage.saveInvoiceOrder(1001, 'order-from-map')
    const id = await resolveOrderId('1001')
    expect(id).toBe('order-from-map')
    expect(orderApi.getOrderIdByInvoice).not.toHaveBeenCalled()
  })

  it('InvId не в карте, но API возвращает id → возвращает и кэширует', async () => {
    vi.mocked(orderApi.getOrderIdByInvoice).mockResolvedValue({ id: 'order-from-api' })
    const id = await resolveOrderId('1002')
    expect(id).toBe('order-from-api')
    expect(orderStorage.getOrderIdByInvoice(1002)).toBe('order-from-api')
  })

  it('InvId передан, но API упал → возвращает null, НЕ откатывается на последний заказ', async () => {
    orderStorage.saveOrder('last-unrelated-order', 'tok') // «последний» заказ в storage
    vi.mocked(orderApi.getOrderIdByInvoice).mockRejectedValue(new Error('network'))
    const id = await resolveOrderId('1003')
    expect(id).toBeNull() // ключевой инвариант P0-4: не подставляем чужой заказ
  })

  it('InvId отсутствует → допустим откат на последний сохранённый заказ', async () => {
    orderStorage.saveOrder('last-order', 'tok')
    const id = await resolveOrderId(null)
    expect(id).toBe('last-order')
    expect(orderApi.getOrderIdByInvoice).not.toHaveBeenCalled()
  })
})
