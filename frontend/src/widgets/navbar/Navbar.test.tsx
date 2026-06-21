import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { Navbar } from './Navbar'

describe('Navbar', () => {
  it('рендерит логотип и ссылку на статус заказа', () => {
    render(
      <MemoryRouter>
        <Navbar />
      </MemoryRouter>,
    )

    expect(screen.getByText(/Numaestra/)).toBeInTheDocument()
    expect(screen.getByText('Мой заказ')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Мой заказ' })).toHaveAttribute('href', '/status')
  })
})
