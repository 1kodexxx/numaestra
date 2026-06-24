import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { NotFoundPage } from './NotFoundPage'
import { renderWithRouter } from '@test/renderWithRouter'

vi.mock('@shared/lib/seo', () => ({
  useSeo: () => {},
}))

describe('NotFoundPage', () => {
  it('рендерит 404 и кнопки навигации', async () => {
    const user = userEvent.setup()
    renderWithRouter(<NotFoundPage />, { route: '/missing-page' })

    expect(screen.getByText('404')).toBeInTheDocument()
    expect(screen.getByText('Такой страницы нет')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'На главную' }))
    await user.click(screen.getByRole('button', { name: 'Мой заказ' }))
  })
})
