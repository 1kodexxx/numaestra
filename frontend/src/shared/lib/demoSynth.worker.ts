// Web Worker: синтез демо-трека вне основного потока, чтобы не фризить UI на
// слабых устройствах. Тяжёлая математика — в renderWavBuffer (чистое ядро).
import { renderWavBuffer } from './demoSynthCore'

const ctx = self as unknown as {
  onmessage: ((e: MessageEvent) => void) | null
  postMessage: (message: unknown, transfer?: Transferable[]) => void
}

ctx.onmessage = (e: MessageEvent) => {
  const { id, seed, seconds } = e.data as { id: number; seed: number; seconds?: number }
  try {
    const buf = renderWavBuffer(seed, seconds ?? 16)
    ctx.postMessage({ id, ok: true, buf }, [buf])
  } catch {
    ctx.postMessage({ id, ok: false })
  }
}
