import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AdminOrderDetailPage } from './AdminOrderDetailPage'
import { adminOrderApi } from '@entities/admin-order'
import type { AdminOrder } from '@entities/admin-order'
import { ORDER_ID, makeTracks } from '@test/fixtures/orders'
import { renderWithRouter } from '@test/renderWithRouter'
import { ApiError } from '@shared/api'

const navigateMock = vi.fn()

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => navigateMock,
  }
})

vi.mock('@entities/admin-order', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@entities/admin-order')>()
  return {
    ...actual,
    adminOrderApi: {
      ...actual.adminOrderApi,
      get: vi.fn(),
      remove: vi.fn(),
      refund: vi.fn(),
      regenerate: vi.fn(),
      sendFeedback: vi.fn(),
    },
  }
})

vi.mock('@shared/lib/seo', () => ({
  useSeo: () => {},
}))

function adminOrder(): AdminOrder {
  return {
    id: ORDER_ID,
    invoice_id: 4242,
    email: 'user@example.com',
    phone: '',
    brief: 'Тестовый бриф',
    amount_kopecks: 200_000,
    payment_status: 'paid',
    generation_status: 'completed',
    tracks: makeTracks(),
    created_at: new Date().toISOString(),
  }
}

describe('AdminOrderDetailPage', () => {
  beforeEach(() => {
    navigateMock.mockReset()
    vi.mocked(adminOrderApi.get).mockReset()
    vi.mocked(adminOrderApi.remove).mockReset()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  it('показывает блок удаления после загрузки заказа', async () => {
    vi.mocked(adminOrderApi.get).mockResolvedValue(adminOrder())

    renderWithRouter(<AdminOrderDetailPage />, {
      route: `/admin/orders/${ORDER_ID}`,
      path: '/admin/orders/:id',
    })

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Удалить заказ' })).toBeInTheDocument()
    })
    expect(screen.getByText(/Полностью удалить заказ из базы/)).toBeInTheDocument()
  })

  it('после подтверждения удаляет заказ и уходит к списку', async () => {
    vi.mocked(adminOrderApi.get).mockResolvedValue(adminOrder())
    vi.mocked(adminOrderApi.remove).mockResolvedValue(undefined)

    renderWithRouter(<AdminOrderDetailPage />, {
      route: `/admin/orders/${ORDER_ID}`,
      path: '/admin/orders/:id',
    })

    const user = userEvent.setup()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Удалить заказ' })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: 'Удалить заказ' }))

    await waitFor(() => {
      expect(adminOrderApi.remove).toHaveBeenCalledWith(ORDER_ID)
      expect(navigateMock).toHaveBeenCalledWith('/admin/orders')
    })
  })

  it('не удаляет заказ, если confirm отменён', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    vi.mocked(adminOrderApi.get).mockResolvedValue(adminOrder())

    renderWithRouter(<AdminOrderDetailPage />, {
      route: `/admin/orders/${ORDER_ID}`,
      path: '/admin/orders/:id',
    })

    const user = userEvent.setup()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Удалить заказ' })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: 'Удалить заказ' }))

    expect(adminOrderApi.remove).not.toHaveBeenCalled()
    expect(navigateMock).not.toHaveBeenCalled()
  })

  it('показывает ошибку, если удаление не удалось', async () => {
    vi.mocked(adminOrderApi.get).mockResolvedValue(adminOrder())
    vi.mocked(adminOrderApi.remove).mockRejectedValue(new ApiError(500, 'storage unavailable'))

    renderWithRouter(<AdminOrderDetailPage />, {
      route: `/admin/orders/${ORDER_ID}`,
      path: '/admin/orders/:id',
    })

    const user = userEvent.setup()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Удалить заказ' })).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: 'Удалить заказ' }))

    expect(await screen.findByText('storage unavailable')).toBeInTheDocument()
    expect(navigateMock).not.toHaveBeenCalled()
  })
})
