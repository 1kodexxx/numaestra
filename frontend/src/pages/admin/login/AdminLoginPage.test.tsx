import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AdminLoginPage } from './AdminLoginPage'
import { AdminSessionProvider } from '@features/admin-session'

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/admin/login']}>
      <AdminSessionProvider>
        <AdminLoginPage />
      </AdminSessionProvider>
    </MemoryRouter>,
  )
}

describe('AdminLoginPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('блокирует кнопку входа, пока поля не заполнены', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{"login":""}', { status: 401 })))
    renderPage()

    await waitFor(() => expect(screen.getByRole('button', { name: /войти/i })).toBeDisabled())
  })

  it('отправляет логин/пароль и показывает ошибку при неверных данных', async () => {
    const fetchMock = vi.fn()
      // 1-й вызов — проверка сессии при монтировании (AdminSessionProvider)
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: 'unauthorized' }), { status: 401 }))
      // 2-й вызов — сам логин
      .mockResolvedValueOnce(new Response(JSON.stringify({ error: 'неверный логин или пароль' }), { status: 401 }))
    vi.stubGlobal('fetch', fetchMock)

    renderPage()
    const user = userEvent.setup()

    await user.type(screen.getByLabelText('Логин'), 'owner')
    await user.type(screen.getByLabelText('Пароль'), 'wrong-password')
    await user.click(screen.getByRole('button', { name: /войти/i }))

    expect(await screen.findByText('неверный логин или пароль')).toBeInTheDocument()
  })
})
