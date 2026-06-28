import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DemoPlayer } from './DemoPlayer'

describe('DemoPlayer — состояние «готовим демо»', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('показывает живой таймер, ползущий процент и эскалирует ободрение', () => {
    render(<DemoPlayer status="processing" />)

    // Стартовое состояние: таймер 0:00, низкий процент, мягкое «пара минут».
    expect(screen.getByText('0:00')).toBeInTheDocument()
    expect(screen.getByText('5%')).toBeInTheDocument()
    expect(screen.getByText('Обычно занимает пару минут')).toBeInTheDocument()

    // Процент ползёт вперёд — спустя 30 секунд он явно больше стартового.
    act(() => {
      vi.advanceTimersByTime(30_000)
    })
    expect(screen.queryByText('5%')).not.toBeInTheDocument()

    // Спустя ~2.5 минуты: таймер идёт, бар у потолка (не застрял на 100),
    // и текст явно успокаивает — не зависли, обновится само.
    act(() => {
      vi.advanceTimersByTime(130_000)
    })
    expect(screen.getByText('2:40')).toBeInTheDocument()
    expect(screen.getByText('96%')).toBeInTheDocument()
    expect(screen.getByText(/Мы не зависли/)).toBeInTheDocument()
  })
})
