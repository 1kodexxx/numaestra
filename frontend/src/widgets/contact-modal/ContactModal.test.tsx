import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ContactModal } from './ContactModal'

describe('ContactModal', () => {
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
