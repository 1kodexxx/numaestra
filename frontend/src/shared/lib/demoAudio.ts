// Генерация демо-звучания примеров. Тяжёлый синтез вынесен в Web Worker
// (demoSynth.worker), чтобы не блокировать UI-поток на слабых устройствах.
// Если воркер недоступен — фолбэк на синхронный рендер на основном потоке.
import { renderWavBuffer } from './demoSynthCore'

/** Детерминированный хэш строки → число (стабильное «звучание» примера). */
export function hashStr(s: string): number {
  let h = 5381
  for (let i = 0; i < s.length; i++) h = ((h << 5) + h + s.charCodeAt(i)) >>> 0
  return h
}

let worker: Worker | null = null
let workerBroken = false
let nextId = 1
const pending = new Map<number, (buf: ArrayBuffer | null) => void>()

function getWorker(): Worker | null {
  if (workerBroken) return null
  if (worker) return worker
  try {
    worker = new Worker(new URL('./demoSynth.worker.ts', import.meta.url), { type: 'module' })
    worker.onmessage = (e: MessageEvent) => {
      const { id, ok, buf } = e.data as { id: number; ok: boolean; buf?: ArrayBuffer }
      const resolve = pending.get(id)
      if (resolve) { pending.delete(id); resolve(ok && buf ? buf : null) }
    }
    worker.onerror = () => {
      workerBroken = true
      worker = null
      pending.forEach((r) => r(null))
      pending.clear()
    }
  } catch {
    workerBroken = true
    worker = null
  }
  return worker
}

/**
 * Синтезирует демо-трек. seed задаёт тональность/мажор-минор, чтобы примеры и
 * варианты звучали по-разному. Возвращает blob URL и длительность (сек).
 */
export async function synthDemoTrack(seed: number, seconds = 16): Promise<{ url: string; duration: number } | null> {
  let buf: ArrayBuffer | null = null

  const w = getWorker()
  if (w) {
    buf = await new Promise<ArrayBuffer | null>((resolve) => {
      const id = nextId++
      pending.set(id, resolve)
      try { w.postMessage({ id, seed, seconds }) }
      catch { pending.delete(id); resolve(null) }
      // Страховка: если воркер завис — уходим в фолбэк.
      setTimeout(() => { if (pending.has(id)) { pending.delete(id); resolve(null) } }, 10000)
    })
  }

  if (!buf) {
    try { buf = renderWavBuffer(seed, seconds) } catch { return null }
  }
  if (!buf) return null

  return { url: URL.createObjectURL(new Blob([buf], { type: 'audio/wav' })), duration: seconds }
}

/* ── кэш готовых демо-треков на сессию (примеров немного) ── */
const cache = new Map<number, { url: string; duration: number }>()

/**
 * Прогрев: заранее (в фоне через воркер) генерирует демо-трек и кладёт в кэш.
 * Чтобы к моменту тапа по примеру URL уже был готов и воспроизведение
 * запускалось синхронно внутри пользовательского жеста (важно для мобильных,
 * где автоплей после `await` блокируется).
 */
export function prewarmDemoTrack(seed: number, seconds = 16): void {
  if (cache.has(seed)) return
  void synthDemoTrack(seed, seconds).then((t) => {
    if (!t) return
    if (cache.has(seed)) URL.revokeObjectURL(t.url) // гонка прогревов
    else cache.set(seed, t)
  })
}

/**
 * Синхронно возвращает демо-трек: из кэша (если прогрев успел) либо рендерит
 * на месте. Синхронность критична: вызов в обработчике тапа сохраняет
 * пользовательский жест, и последующий audio.play() не блокируется
 * автоплей-политикой мобильных браузеров.
 */
export function getDemoTrackSync(seed: number, seconds = 16): { url: string; duration: number } | null {
  const hit = cache.get(seed)
  if (hit) return hit
  let buf: ArrayBuffer | null = null
  try { buf = renderWavBuffer(seed, seconds) } catch { return null }
  if (!buf) return null
  const t = { url: URL.createObjectURL(new Blob([buf], { type: 'audio/wav' })), duration: seconds }
  cache.set(seed, t)
  return t
}
