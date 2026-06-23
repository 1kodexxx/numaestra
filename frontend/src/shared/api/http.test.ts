import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch, ApiError } from './http'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('apiFetch', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('обращается к /api/v1 + path и парсит JSON-ответ', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ id: 'order-1' }))

    const result = await apiFetch<{ id: string }>('/orders/order-1')

    expect(result).toEqual({ id: 'order-1' })
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/orders/order-1', expect.any(Object))
  })

  it('сериализует body в JSON и ставит Content-Type', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ ok: true }))

    await apiFetch('/orders/', { method: 'POST', body: { email: 'a@b.c' } })

    const [, init] = fetchMock.mock.calls[0]
    expect(init.method).toBe('POST')
    expect(init.body).toBe(JSON.stringify({ email: 'a@b.c' }))
    expect(init.headers['Content-Type']).toBe('application/json')
    // cookie-сессия админки требует credentials
    expect(init.credentials).toBe('include')
  })

  it('добавляет X-Access-Token, когда передан accessToken', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await apiFetch('/orders/order-1', { accessToken: 'secret-token' })

    const [, init] = fetchMock.mock.calls[0]
    expect(init.headers['X-Access-Token']).toBe('secret-token')
  })

  it('не добавляет X-Access-Token без токена', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({}))

    await apiFetch('/orders/')

    const [, init] = fetchMock.mock.calls[0]
    expect(init.headers['X-Access-Token']).toBeUndefined()
  })

  it('бросает ApiError с сообщением из тела при не-2xx', async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ error: 'заказ не найден', request_id: 'req-42' }, 404),
    )

    await expect(apiFetch('/orders/missing')).rejects.toMatchObject({
      name: 'ApiError',
      status: 404,
      message: 'заказ не найден',
      requestId: 'req-42',
    })
  })

  it('использует дефолтное сообщение, если тело ошибки не JSON', async () => {
    fetchMock.mockResolvedValueOnce(new Response('gateway down', { status: 502 }))

    await expect(apiFetch('/orders/')).rejects.toMatchObject({
      status: 502,
      message: 'HTTP 502',
    })
  })

  it('возвращает пустой объект на 204 No Content', async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))

    await expect(apiFetch('/orders/order-1')).resolves.toEqual({})
  })

  it('ApiError — настоящий Error с полем status', () => {
    const err = new ApiError(403, 'forbidden')
    expect(err).toBeInstanceOf(Error)
    expect(err.status).toBe(403)
  })
})
