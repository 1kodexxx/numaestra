import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { isChunkLoadError, reloadOnceForChunkError } from './lazyWithReload'

describe('isChunkLoadError', () => {
  it('распознаёт типичные ошибки загрузки чанка (разные браузеры/бандлеры)', () => {
    expect(isChunkLoadError(new Error('Failed to fetch dynamically imported module: /assets/x.js'))).toBe(true)
    expect(isChunkLoadError(new Error('Importing a module script failed.'))).toBe(true)
    expect(isChunkLoadError(new Error('error loading dynamically imported module'))).toBe(true)
    const e = new Error('boom'); e.name = 'ChunkLoadError'
    expect(isChunkLoadError(e)).toBe(true)
  })

  it('не считает обычные баги кода чанк-ошибкой', () => {
    expect(isChunkLoadError(new Error("Cannot read properties of undefined (reading 'x')"))).toBe(false)
    expect(isChunkLoadError(new TypeError('x is not a function'))).toBe(false)
    expect(isChunkLoadError('просто строка')).toBe(false)
  })
})

describe('reloadOnceForChunkError', () => {
  const reload = vi.fn()

  beforeEach(() => {
    sessionStorage.clear()
    reload.mockClear()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...window.location, reload },
    })
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('перезагружает один раз при чанк-ошибке и возвращает true', () => {
    const ok = reloadOnceForChunkError(new Error('Failed to fetch dynamically imported module'))
    expect(ok).toBe(true)
    expect(reload).toHaveBeenCalledTimes(1)
  })

  it('не перезагружается повторно сразу (защита от петли)', () => {
    reloadOnceForChunkError(new Error('Importing a module script failed'))
    reload.mockClear()
    const second = reloadOnceForChunkError(new Error('Importing a module script failed'))
    expect(second).toBe(false)
    expect(reload).not.toHaveBeenCalled()
  })

  it('не трогает не-чанковые ошибки', () => {
    const ok = reloadOnceForChunkError(new Error('Cannot read properties of undefined'))
    expect(ok).toBe(false)
    expect(reload).not.toHaveBeenCalled()
  })
})
