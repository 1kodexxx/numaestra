import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { StatusPage } from './StatusPage'
import { orderApi } from '@entities/order'
import { orderStorage } from '@shared/lib/storage'
import { renderWithRouter } from '@test/renderWithRouter'
import {
  ACCESS_TOKEN,
  ORDER_ID,
  completedOrder,
  pendingOrder,
  processingOrder,
} from '@test/fixtures/orders'

vi.mock('@entities/order', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@entities/order')>()
  return {
    ...actual,
    orderApi: {
      ...actual.orderApi,
      getById: vi.fn(),
      paymentUrl: vi.fn(),
      list: vi.fn(),
    },
  }
})

vi.mock('@shared/lib/seo', () => ({
  useSeo: () => {},
}))

vi.mock('@shared/lib/download', () => ({
  downloadFile: vi.fn(),
}))

describe('StatusPage', () => {
  beforeAll(() => {
    vi.spyOn(window.HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    vi.spyOn(window.HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  })

  beforeEach(() => {
    localStorage.clear()
    vi.mocked(orderApi.getById).mockReset()
    vi.mocked(orderApi.paymentUrl).mockReset()
    vi.mocked(orderApi.list).mockResolvedValue([])
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('показывает форму поиска без активного заказа', () => {
    renderWithRouter(<StatusPage />, { route: '/status' })

    expect(screen.getByText('Введите ID вашего заказа')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Найти' })).toBeInTheDocument()
    expect(orderApi.getById).not.toHaveBeenCalled()
  })

  it('сохраняет order_id и token из URL и показывает готовый заказ с 4 треками', async () => {
    vi.mocked(orderApi.getById).mockResolvedValue(completedOrder())

    renderWithRouter(<StatusPage />, {
      route: `/status/${ORDER_ID}?token=${ACCESS_TOKEN}`,
      path: '/status/:orderId',
    })

    await waitFor(() => {
      expect(screen.getByText('Ваша песня готова!')).toBeInTheDocument()
    })

    expect(orderStorage.getOrderId()).toBe(ORDER_ID)
    expect(orderStorage.getAccessToken()).toBe(ACCESS_TOKEN)
    expect(orderApi.getById).toHaveBeenCalledWith(ORDER_ID, ACCESS_TOKEN)
    expect(screen.getByText('Скачать треки')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /Вариант [1-4]/ })).toHaveLength(8)
  })

  it('показывает ожидание оплаты и переход к Robokassa', async () => {
    vi.mocked(orderApi.getById).mockResolvedValue(pendingOrder())
    vi.mocked(orderApi.paymentUrl).mockResolvedValue({
      payment_url: 'https://auth.robokassa.ru/pay/retry',
    })

    const hrefSpy = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { set href(v: string) { hrefSpy(v) } },
    })

    orderStorage.saveOrder(ORDER_ID, ACCESS_TOKEN)
    renderWithRouter(<StatusPage />, { route: '/status' })

    await waitFor(() => {
      expect(screen.getByText('Ожидание оплаты')).toBeInTheDocument()
    })

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: /Перейти к оплате/ }))

    expect(orderApi.paymentUrl).toHaveBeenCalledWith(ORDER_ID, ACCESS_TOKEN)
    expect(hrefSpy).toHaveBeenCalledWith('https://auth.robokassa.ru/pay/retry')
  })

  it('показывает прогресс генерации после оплаты', async () => {
    vi.mocked(orderApi.getById).mockResolvedValue(processingOrder())
    orderStorage.saveOrder(ORDER_ID, ACCESS_TOKEN)

    renderWithRouter(<StatusPage />, { route: '/status' })

    await waitFor(() => {
      expect(screen.getByText('Создаём песню')).toBeInTheDocument()
    })
    expect(screen.getByText(/%/)).toBeInTheDocument()
  })

  it('legacy query order_id продолжает работать', async () => {
    vi.mocked(orderApi.getById).mockResolvedValue(completedOrder())

    renderWithRouter(<StatusPage />, {
      route: `/status?order_id=${ORDER_ID}&token=${ACCESS_TOKEN}`,
    })

    await waitFor(() => {
      expect(screen.getByText('Ваша песня готова!')).toBeInTheDocument()
    })
    expect(orderApi.getById).toHaveBeenCalledWith(ORDER_ID, ACCESS_TOKEN)
  })

  it('не подставляет старый заказ из localStorage при битой ссылке из письма', async () => {
    orderStorage.saveOrder('old-first-order-id', 'old-token')
    vi.mocked(orderApi.getById).mockResolvedValue(completedOrder())

    renderWithRouter(<StatusPage />, {
      route: `/status?token=${ACCESS_TOKEN}`,
    })

    expect(screen.getByText(/Ссылка из письма открылась некорректно/)).toBeInTheDocument()
    expect(orderApi.getById).not.toHaveBeenCalled()
  })

  it('находит заказ по введённому ID', async () => {
    vi.mocked(orderApi.getById).mockResolvedValue(completedOrder())

    renderWithRouter(<StatusPage />, { route: '/status' })
    const user = userEvent.setup()

    await user.type(screen.getByLabelText('ID заказа'), ORDER_ID)
    await user.click(screen.getByRole('button', { name: 'Найти' }))

    await waitFor(() => {
      expect(screen.getByText('Ваша песня готова!')).toBeInTheDocument()
    })
    expect(orderApi.getById).toHaveBeenCalledWith(ORDER_ID, undefined)
  })
})
