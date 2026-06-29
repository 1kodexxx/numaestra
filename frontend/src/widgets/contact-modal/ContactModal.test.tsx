import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ContactModal } from './ContactModal'

describe('ContactModal', () => {
  it('предлагает исправить опечатку в домене и применяет исправление по клику', () => {
    const base = { loading: false, error: null, priceLabel: '2000 ₽', onSubmit: vi.fn(), onClose: () => {} }
    render(<ContactModal {...base} />)

    const email = screen.getByLabelText(/Email/) as HTMLInputElement
    fireEvent.change(email, { target: { value: 'ivan@gmial.com' } })

    const fix = screen.getByRole('button', { name: /gmail\.com/ })
    expect(fix).toBeInTheDocument()

    fireEvent.click(fix)
    expect(email.value).toBe('ivan@gmail.com')
    // После исправления подсказка пропадает.
    expect(screen.queryByRole('button', { name: /Возможно/ })).not.toBeInTheDocument()
  })

  it('показывает цену со скидкой и экономию при применённом промокоде', () => {
    render(
      <ContactModal
        loading={false}
        error={null}
        priceLabel="2 000 ₽"
        priceKopecks={200_000}
        discountKopecks={50_000}
        discountLabel="−25%"
        onClose={() => {}}
        onSubmit={vi.fn()}
      />,
    )
    // строка экономии (та же сумма уйдёт в оплату)
    const saving = screen.getByText(/сэкономите/)
    expect(saving).toHaveTextContent('500')
    expect(saving).toHaveTextContent('−25%')
    // полная цена перечёркнута, итоговая со скидкой — 1 500
    expect(screen.getByText('2 000 ₽')).toBeInTheDocument()
    expect(screen.getByText(/1\s+500/)).toBeInTheDocument()
  })

  it('без промокода показывает полную цену без экономии', () => {
    render(
      <ContactModal
        loading={false}
        error={null}
        priceLabel="2 000 ₽"
        priceKopecks={200_000}
        onClose={() => {}}
        onSubmit={vi.fn()}
      />,
    )
    expect(screen.getByText('2 000 ₽')).toBeInTheDocument()
    expect(screen.queryByText(/сэкономите/)).not.toBeInTheDocument()
  })

  it('показывает подтверждение адреса для корректного email', () => {
    const base = { loading: false, error: null, priceLabel: '2000 ₽', onSubmit: vi.fn(), onClose: () => {} }
    render(<ContactModal {...base} />)

    fireEvent.change(screen.getByLabelText(/Email/), { target: { value: 'ivan@gmail.com' } })
    expect(screen.getByText(/придут на/)).toBeInTheDocument()
    expect(screen.getByText('ivan@gmail.com')).toBeInTheDocument()
  })

  it('не теряет фокус инпута при ре-рендере родителя (фикс мобильной клавиатуры)', () => {
    const base = { loading: false, error: null, priceLabel: '2000 ₽', onSubmit: vi.fn() }
    // onClose — инлайн-функция, как в CatalogPage: на каждый ре-рендер новая ссылка.
    const { rerender } = render(<ContactModal {...base} onClose={() => {}} />)

    const email = screen.getByLabelText(/Email/) as HTMLInputElement
    email.focus()
    expect(document.activeElement).toBe(email)

    // Родитель ре-рендерится и передаёт НОВУЮ onClose. Эффект фокуса не должен
    // перезапуститься и увести фокус на контейнер — иначе на мобиле клавиатура
    // мгновенно закрывается.
    rerender(<ContactModal {...base} onClose={() => {}} />)

    expect(document.activeElement).toBe(email)
  })
})
