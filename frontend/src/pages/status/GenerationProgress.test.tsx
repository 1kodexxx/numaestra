import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { GenerationProgress } from './GenerationProgress'

describe('GenerationProgress', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('показывает 100% и финальный текст при completed', () => {
    render(<GenerationProgress status="completed" />)

    expect(screen.getByText('100%')).toBeInTheDocument()
    expect(screen.getByText('Песня готова 🎉')).toBeInTheDocument()
  })

  it('не достигает 100% до фактического завершения (потолок 94%)', () => {
    // paid_at далеко в прошлом → кривая упёрлась в CAP, но не в 100%.
    const longAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString()
    render(<GenerationProgress status="processing" paidAt={longAgo} />)

    const pct = Number(screen.getByText(/%$/).textContent!.replace('%', ''))
    expect(pct).toBeGreaterThan(0)
    expect(pct).toBeLessThanOrEqual(94)
  })

  it('стартует около 0% сразу после оплаты', () => {
    render(<GenerationProgress status="queued" paidAt={new Date().toISOString()} />)

    expect(screen.getByText('0%')).toBeInTheDocument()
  })

  it('в статусе queued показывает сообщение фазы очереди', () => {
    render(<GenerationProgress status="queued" paidAt={new Date().toISOString()} />)

    expect(screen.getByText('Заказ принят — становимся в очередь…')).toBeInTheDocument()
  })

  it('растёт со временем без обновления статуса', () => {
    render(<GenerationProgress status="processing" paidAt={new Date().toISOString()} />)
    expect(screen.getByText('0%')).toBeInTheDocument()

    // Прокручиваем 2 минуты ежесекундных тиков (setNow внутри — оборачиваем в act).
    act(() => { vi.advanceTimersByTime(120_000) })

    const pct = Number(screen.getByText(/%$/).textContent!.replace('%', ''))
    expect(pct).toBeGreaterThan(0)
  })
})
