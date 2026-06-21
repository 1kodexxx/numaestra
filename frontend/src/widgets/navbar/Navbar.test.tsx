import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import { Navbar } from './Navbar'

describe('Navbar', () => {
  it('рендерит ссылки на каталог и статус заказа', () => {
    render(
      <MemoryRouter>
        <Navbar />
      </MemoryRouter>,
    )

    expect(screen.getByText(/Numaestra/)).toBeInTheDocument()
    expect(screen.getByText('Каталог')).toBeInTheDocument()
    expect(screen.getByText('Мой заказ')).toBeInTheDocument()
  })
})
