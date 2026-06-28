import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { adminOrderApi } from './api'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('adminOrderApi', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('get запрашивает заказ по id', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ id: 'order-1', invoice_id: 1 }))

    const order = await adminOrderApi.get('order-1')

    expect(order.id).toBe('order-1')
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/orders/order-1', expect.any(Object))
  })

  it('remove отправляет DELETE на /admin/orders/{id}', async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))

    await adminOrderApi.remove('order-uuid')

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/orders/order-uuid', expect.objectContaining({ method: 'DELETE' }))
  })

  it('refund отправляет POST', async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))

    await adminOrderApi.refund('order-uuid')

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/admin/orders/order-uuid/refund')
    expect(init.method).toBe('POST')
  })

  it('deleteDemo отправляет DELETE на /admin/orders/{id}/demo', async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))

    await adminOrderApi.deleteDemo('order-uuid')

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/orders/order-uuid/demo', expect.objectContaining({ method: 'DELETE' }))
  })
})
