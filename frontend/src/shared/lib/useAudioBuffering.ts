import { useEffect, useState } from 'react'

/**
 * Отслеживает буферизацию <audio>, чтобы плеер показывал индикатор загрузки,
 * пока звук ещё подгружается. Возвращает [buffering, setBuffering]: обработчик
 * нажатия play вызывает setBuffering(true) для мгновенной обратной связи, а
 * событие `playing` (реальный старт звука) само снимает индикатор.
 */
export function useAudioBuffering(
  audioRef: React.RefObject<HTMLAudioElement | null>,
): readonly [boolean, (v: boolean) => void] {
  const [buffering, setBuffering] = useState(false)

  useEffect(() => {
    const el = audioRef.current
    if (!el) return
    const on = () => setBuffering(true)   // ждём данные (waiting/stalled)
    const off = () => setBuffering(false) // звук пошёл или остановлен
    el.addEventListener('waiting', on)
    el.addEventListener('stalled', on)
    el.addEventListener('playing', off)
    el.addEventListener('pause', off)
    el.addEventListener('ended', off)
    el.addEventListener('error', off)
    return () => {
      el.removeEventListener('waiting', on)
      el.removeEventListener('stalled', on)
      el.removeEventListener('playing', off)
      el.removeEventListener('pause', off)
      el.removeEventListener('ended', off)
      el.removeEventListener('error', off)
    }
  }, [audioRef])

  return [buffering, setBuffering] as const
}
