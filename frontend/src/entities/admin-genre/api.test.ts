import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { adminGenreApi } from './api'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('adminGenreApi', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('list запрашивает /admin/genres/', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([]))

    await adminGenreApi.list()

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/genres/', expect.any(Object))
  })

  it('create отправляет POST с телом жанра', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ id: 1, slug: 'pop', label: 'Поп', suno_value: 'modern pop' }))

    await adminGenreApi.create({ slug: 'pop', label: 'Поп', suno_value: 'modern pop', sort_order: 10 })

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/admin/genres/')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body)).toEqual({
      slug: 'pop',
      label: 'Поп',
      suno_value: 'modern pop',
      sort_order: 10,
    })
  })

  it('setCategoryGenres отправляет genre_ids', async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }))

    await adminGenreApi.setCategoryGenres('wedding', [1, 2])

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/v1/admin/categories/wedding/genres')
    expect(init.method).toBe('PUT')
    expect(JSON.parse(init.body)).toEqual({ genre_ids: [1, 2] })
  })
})
