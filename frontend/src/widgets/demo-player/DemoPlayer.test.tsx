import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DemoPlayer } from './DemoPlayer'

describe('DemoPlayer — состояние «готовим демо»', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('живой таймер, растущий процент и эскалация ободрения', () => {
    render(<DemoPlayer status="processing" />)

    // Старт: таймер 0:00, низкий процент, мягкое «пара минут».
    expect(screen.getByText('0:00')).toBeInTheDocument()
    expect(screen.getByText('3%')).toBeInTheDocument()
    expect(screen.getByText('Обычно занимает пару минут')).toBeInTheDocument()

    // Процент растёт — спустя 30 секунд он явно больше стартового.
    act(() => {
      vi.advanceTimersByTime(30_000)
    })
    expect(screen.queryByText('3%')).not.toBeInTheDocument()

    // Спустя ~2.5 мин: таймер идёт, бар не на 100% (честно), текст успокаивает.
    act(() => {
      vi.advanceTimersByTime(130_000)
    })
    expect(screen.getByText('2:40')).toBeInTheDocument()
    expect(screen.queryByText('100%')).not.toBeInTheDocument()
    expect(screen.getByText(/Мы не зависли/)).toBeInTheDocument()
  })

  it('анкорится на серверный старт: прогресс не сбрасывается при перезагрузке', () => {
    // Демо стартовало 2 минуты назад (серверный updated_at). Свежий монтаж
    // (как после reload) должен показать прогресс ~2 минуты, а не 0.
    const startedAt = new Date(Date.now() - 120_000).toISOString()
    render(<DemoPlayer status="processing" startedAt={startedAt} />)

    expect(screen.getByText('2:00')).toBeInTheDocument()
    // Не сброшено в начало — стартовых 3% / 0:00 нет.
    expect(screen.queryByText('3%')).not.toBeInTheDocument()
    expect(screen.queryByText('0:00')).not.toBeInTheDocument()
  })
})
