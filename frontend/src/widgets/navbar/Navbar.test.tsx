import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { Navbar } from './Navbar'

function setViewportWidth(width: number) {
  Object.defineProperty(window, 'innerWidth', { writable: true, configurable: true, value: width })
  window.dispatchEvent(new Event('resize'))
}

describe('Navbar', () => {
  const originalWidth = window.innerWidth

  afterEach(() => {
    setViewportWidth(originalWidth)
    document.body.style.overflow = ''
  })

  it('рендерит логотип и ссылку на статус заказа', () => {
    setViewportWidth(1280)
    render(
      <MemoryRouter>
        <Navbar />
      </MemoryRouter>,
    )

    expect(screen.getByText(/Numaestra/)).toBeInTheDocument()
    expect(screen.getByText('Мой заказ')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Мой заказ' })).toHaveAttribute('href', '/status')
  })

  it('на десктопе показывает вторичные ссылки в шапке', () => {
    setViewportWidth(1280)
    render(
      <MemoryRouter>
        <Navbar />
      </MemoryRouter>,
    )

    expect(screen.getByRole('link', { name: 'Как это работает' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Отзывы' })).toBeInTheDocument()
  })

  describe('mobile menu', () => {
    beforeEach(() => {
      setViewportWidth(390)
    })

    it('показывает кнопку меню и скрывает вторичные ссылки в шапке', () => {
      render(
        <MemoryRouter>
          <Navbar />
        </MemoryRouter>,
      )

      expect(screen.getByRole('button', { name: 'Открыть меню' })).toBeInTheDocument()
      expect(screen.queryByRole('link', { name: 'Как это работает' })).not.toBeInTheDocument()
      expect(screen.queryByRole('link', { name: 'Отзывы' })).not.toBeInTheDocument()
    })

    it('открывает лист навигации со всеми пунктами', async () => {
      const user = userEvent.setup()
      render(
        <MemoryRouter>
          <Navbar />
        </MemoryRouter>,
      )

      await user.click(screen.getByRole('button', { name: 'Открыть меню' }))

      expect(screen.getByRole('dialog', { name: 'Навигация' })).toBeInTheDocument()
      expect(screen.getByRole('link', { name: /Каталог/ })).toHaveAttribute('href', '/')
      expect(screen.getByRole('link', { name: /Как это работает/ })).toHaveAttribute('href', '/how-it-works')
      expect(screen.getByRole('link', { name: /Отзывы/ })).toHaveAttribute('href', '/reviews')
      expect(screen.getAllByRole('link', { name: 'Мой заказ' })).toHaveLength(1)
    })

    it('закрывает меню по Escape', async () => {
      const user = userEvent.setup()
      render(
        <MemoryRouter>
          <Navbar />
        </MemoryRouter>,
      )

      await user.click(screen.getByRole('button', { name: 'Открыть меню' }))
      expect(screen.getByRole('dialog', { name: 'Навигация' })).toBeInTheDocument()

      await user.keyboard('{Escape}')
      expect(screen.queryByRole('dialog', { name: 'Навигация' })).not.toBeInTheDocument()
    })
  })
})
