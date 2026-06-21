import '@testing-library/jest-dom/vitest'
import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'

// vite.config.ts не включает test.globals — RTL не находит глобальный afterEach
// и не очищает DOM сама. Без этого элементы из предыдущих тестов в одном файле
// остаются в document.body и ломают getByRole/getByText в следующих тестах.
afterEach(() => {
  cleanup()
})
