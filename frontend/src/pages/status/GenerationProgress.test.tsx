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
    render(<GenerationProgress status="completed" generationProgress={100} />)

    expect(screen.getByText('100%')).toBeInTheDocument()
    expect(screen.getByText('Песня готова 🎉')).toBeInTheDocument()
  })

  it('использует прогресс с сервера вместо чисто временной оценки', () => {
    render(
      <GenerationProgress
        status="processing"
        paidAt={new Date().toISOString()}
        generationPhase="generating"
        generationProgress={62}
        tracksReady={2}
      />,
    )

    expect(screen.getByText('62%')).toBeInTheDocument()
    expect(screen.getByText('🎼 Готово 2 из 4 версий…')).toBeInTheDocument()
  })

  it('не достигает 100% до фактического завершения', () => {
    render(
      <GenerationProgress
        status="processing"
        paidAt={new Date(Date.now() - 60 * 60 * 1000).toISOString()}
        generationProgress={85}
        generationPhase="generating"
      />,
    )

    const pct = Number(screen.getByText(/%$/).textContent!.replace('%', ''))
    expect(pct).toBeGreaterThan(0)
    expect(pct).toBeLessThan(100)
  })

  it('в статусе queued показывает сообщение фазы очереди', () => {
    render(
      <GenerationProgress
        status="queued"
        paidAt={new Date().toISOString()}
        generationPhase="queued"
        generationProgress={3}
      />,
    )

    expect(screen.getByText('3%')).toBeInTheDocument()
    expect(screen.getByText('Заказ принят — становимся в очередь…')).toBeInTheDocument()
  })

  it('плавно подтягивает отображение между опросами', () => {
    render(
      <GenerationProgress
        status="processing"
        generationPhase="generating"
        generationProgress={40}
        paidAt={new Date().toISOString()}
      />,
    )
    expect(screen.getByText('40%')).toBeInTheDocument()

    act(() => { vi.advanceTimersByTime(8_000) })

    const pct = Number(screen.getByText(/%$/).textContent!.replace('%', ''))
    expect(pct).toBeGreaterThanOrEqual(40)
    expect(pct).toBeLessThanOrEqual(43)
  })
})
