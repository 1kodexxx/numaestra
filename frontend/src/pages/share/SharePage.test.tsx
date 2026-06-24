import { screen, waitFor } from '@testing-library/react'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { SharePage } from './SharePage'
import { orderApi } from '@entities/order'
import { renderWithRouter } from '@test/renderWithRouter'
import { ORDER_ID, makeTracks } from '@test/fixtures/orders'

vi.mock('@entities/order', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@entities/order')>()
  return {
    ...actual,
    orderApi: { ...actual.orderApi, getPublicShare: vi.fn() },
  }
})

vi.mock('@shared/lib/seo', () => ({
  useSeo: () => {},
}))

describe('SharePage', () => {
  beforeAll(() => {
    vi.spyOn(window.HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    vi.spyOn(window.HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  })

  beforeEach(() => {
    vi.mocked(orderApi.getPublicShare).mockReset()
  })

  it('показывает плеер с 4 вариантами для публичной ссылки', async () => {
    vi.mocked(orderApi.getPublicShare).mockResolvedValue({
      id: ORDER_ID,
      tracks: makeTracks(4),
    })

    renderWithRouter(<SharePage />, { route: `/s/${ORDER_ID}`, path: '/s/:id' })

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Вариант 4' })).toBeInTheDocument()
    })
    expect(orderApi.getPublicShare).toHaveBeenCalledWith(ORDER_ID)
    expect(screen.getByText('Кто-то поделился с вами песней')).toBeInTheDocument()
  })

  it('показывает ошибку, если песня не найдена', async () => {
    vi.mocked(orderApi.getPublicShare).mockRejectedValue(new Error('не найдена'))

    renderWithRouter(<SharePage />, { route: `/s/${ORDER_ID}`, path: '/s/:id' })

    await waitFor(() => {
      expect(screen.getByText('Песня не найдена')).toBeInTheDocument()
    })
  })
})
