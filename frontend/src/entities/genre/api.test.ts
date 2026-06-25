import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { genreApi } from './api'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('genreApi', () => {
  let fetchMock: ReturnType<typeof vi.fn>

  beforeEach(() => {
    fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('list без category_id запрашивает /genres', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([{ id: 1, slug: 'pop', label: 'Поп', suno_value: 'modern pop' }]))

    const genres = await genreApi.list()

    expect(genres).toHaveLength(1)
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/genres', expect.any(Object))
  })

  it('list с category_id добавляет query-параметр', async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse([]))

    await genreApi.list('wedding')

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/genres?category_id=wedding', expect.any(Object))
  })
})
