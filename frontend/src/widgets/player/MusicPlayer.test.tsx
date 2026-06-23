import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { MusicPlayer } from './MusicPlayer'
import type { Track } from '@entities/order'

const tracks: Track[] = [
  { index: 1, audio_url: 'https://cdn/1.mp3', duration_sec: 180 },
  { index: 2, audio_url: 'https://cdn/2.mp3', duration_sec: 200 },
  { index: 3, audio_url: 'https://cdn/3.mp3', duration_sec: 175 },
  { index: 4, audio_url: 'https://cdn/4.mp3', duration_sec: 190 },
]

describe('MusicPlayer', () => {
  beforeAll(() => {
    // jsdom не реализует медиа-методы — иначе play()/pause() кидают.
    vi.spyOn(window.HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    vi.spyOn(window.HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  })

  beforeEach(() => {
    vi.mocked(window.HTMLMediaElement.prototype.play).mockClear()
  })

  it('рендерит вкладку на каждый из 4 вариантов', () => {
    render(<MusicPlayer tracks={tracks} />)

    expect(screen.getByRole('button', { name: 'Вариант 1' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Вариант 4' })).toBeInTheDocument()
  })

  it('переключает play/pause по кнопке воспроизведения', async () => {
    render(<MusicPlayer tracks={tracks} />)
    const user = userEvent.setup()

    const playBtn = screen.getByRole('button', { name: 'Воспроизвести' })
    await user.click(playBtn)

    expect(screen.getByRole('button', { name: 'Пауза' })).toBeInTheDocument()
    expect(window.HTMLMediaElement.prototype.play).toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Пауза' }))
    expect(screen.getByRole('button', { name: 'Воспроизвести' })).toBeInTheDocument()
  })

  it('блокирует «Предыдущий» на первом треке и «Следующий» на последнем', async () => {
    render(<MusicPlayer tracks={tracks} />)
    const user = userEvent.setup()

    expect(screen.getByRole('button', { name: 'Предыдущий вариант' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Следующий вариант' })).toBeEnabled()

    // Переключаемся на последний вариант — границы инвертируются.
    await user.click(screen.getByRole('button', { name: 'Вариант 4' }))

    expect(screen.getByRole('button', { name: 'Следующий вариант' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Предыдущий вариант' })).toBeEnabled()
  })

  it('загружает audio_url выбранного трека в <audio>', async () => {
    const { container } = render(<MusicPlayer tracks={tracks} />)
    const user = userEvent.setup()
    const audio = container.querySelector('audio') as HTMLAudioElement

    expect(audio.src).toBe('https://cdn/1.mp3')

    await user.click(screen.getByRole('button', { name: 'Вариант 3' }))
    expect(audio.src).toBe('https://cdn/3.mp3')
  })
})
