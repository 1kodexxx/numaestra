import { render, type RenderOptions } from '@testing-library/react'
import { MemoryRouter, Route, Routes, type MemoryRouterProps } from 'react-router-dom'
import type { ReactElement } from 'react'

type Options = {
  route?: string
  path?: string
} & Omit<MemoryRouterProps, 'initialEntries'>

export function renderWithRouter(ui: ReactElement, options: Options = {}, renderOptions?: RenderOptions) {
  const { route = '/', path, ...routerProps } = options
  const element = path
    ? (
        <Routes>
          <Route path={path} element={ui} />
        </Routes>
      )
    : ui

  return render(
    <MemoryRouter initialEntries={[route]} {...routerProps}>
      {element}
    </MemoryRouter>,
    renderOptions,
  )
}
